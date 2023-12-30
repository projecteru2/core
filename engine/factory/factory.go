package factory

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/alphadose/haxmap"
	"github.com/cockroachdb/errors"
	"github.com/panjf2000/ants/v2"

	"github.com/projecteru2/core/engine"
	"github.com/projecteru2/core/engine/docker"
	"github.com/projecteru2/core/engine/fake"
	"github.com/projecteru2/core/engine/mocks/fakeengine"
	"github.com/projecteru2/core/engine/systemd"
	"github.com/projecteru2/core/engine/virt"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/store"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

type factory func(ctx context.Context, config types.Config, nodename, endpoint, ca, cert, key string) (engine.API, error)

var (
	engines = map[string]factory{
		docker.TCPPrefixKey:  docker.MakeClient,
		docker.SockPrefixKey: docker.MakeClient,
		virt.GRPCPrefixKey:   virt.MakeClient,
		systemd.TCPPrefix:    systemd.MakeClient,
		fakeengine.PrefixKey: fakeengine.MakeClient,
	}
	engineCache *EngineCache
)

// EngineCache .
type EngineCache struct {
	cache       *utils.EngineCache
	keysToCheck *haxmap.Map[string, engineParams]
	pool        *ants.PoolWithFunc
	config      types.Config
	sto         store.Store
}

// NewEngineCache .
func NewEngineCache(config types.Config, sto store.Store) *EngineCache {
	pool, _ := utils.NewPool(config.MaxConcurrency)
	return &EngineCache{
		cache:       utils.NewEngineCache(12*time.Hour, 10*time.Minute),
		keysToCheck: haxmap.New[string, engineParams](),
		pool:        pool,
		config:      config,
		sto:         sto,
	}
}

// InitEngineCache init engine cache and start engine cache checker
func InitEngineCache(ctx context.Context, config types.Config, sto store.Store) {
	engineCache = NewEngineCache(config, sto)
	// init the cache, we don't care the return values
	if sto != nil {
		_, _ = engineCache.sto.GetNodesByPod(ctx, &types.NodeFilter{
			All: true,
		})
	}
	go engineCache.CheckAlive(ctx)
	go engineCache.CheckNodeStatus(ctx)
}

// Get .
func (e *EngineCache) Get(key string) engine.API {
	return e.cache.Get(key)
}

// Set .
func (e *EngineCache) Set(params engineParams, client engine.API) {
	e.cache.Set(params.getCacheKey(), client)
	e.keysToCheck.Set(params.getCacheKey(), params)
}

// Delete .
func (e *EngineCache) Delete(key string) {
	e.cache.Delete(key)
}

func (e *EngineCache) checkOneNodeStatus(ctx context.Context, params *engineParams) {
	if e.sto == nil {
		return
	}
	logger := log.WithFunc("engine.factory.checkOneNodeStatus")
	nodename := params.nodename
	cacheKey := params.getCacheKey()
	if _, err := e.sto.GetNodeStatus(ctx, nodename); err != nil && errors.Is(err, types.ErrInvaildCount) {
		logger.Warnf(ctx, "node %s is offline, the cache will be removed", nodename)
		RemoveEngineFromCache(ctx, params.endpoint, params.ca, params.cert, params.key)
	} else {
		logger.Errorf(ctx, err, "engine %+v is unavailable, will be replaced and removed", cacheKey)
		e.cache.Set(cacheKey, &fake.EngineWithErr{DefaultErr: err})
	}
}

// CheckAlive checks if the engine in cache is available
func (e *EngineCache) CheckAlive(ctx context.Context) {
	logger := log.WithFunc("engine.factory.CheckAlive")
	logger.Info(ctx, "check alive starts")
	defer logger.Info(ctx, "check alive ends")
	defer e.pool.Release()
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		paramsChan := make(chan engineParams)
		go func() {
			e.keysToCheck.ForEach(func(k string, v engineParams) bool {
				paramsChan <- v
				return true
			})
			close(paramsChan)
		}()

		wg := &sync.WaitGroup{}
		for params := range paramsChan {
			wg.Add(1)
			params := params
			_ = e.pool.Invoke(func() {
				defer wg.Done()
				cacheKey := params.getCacheKey()
				client := e.cache.Get(cacheKey)
				if client == nil {
					e.cache.Delete(params.getCacheKey())
					e.keysToCheck.Del(cacheKey)
					return
				}
				if _, ok := client.(*fake.EngineWithErr); ok {
					if newClient, err := newEngine(ctx, e.config, params.nodename, params.endpoint, params.ca, params.key, params.cert); err != nil {
						logger.Errorf(ctx, err, "engine %+v is still unavailable", cacheKey)
						// check node status
						e.checkOneNodeStatus(ctx, &params)
					} else {
						e.cache.Set(cacheKey, newClient)
					}
					return
				}
				if err := validateEngine(ctx, client, e.config.ConnectionTimeout); err != nil {
					logger.Errorf(ctx, err, "engine %+v is unavailable, will be replaced and removed", cacheKey)
					e.cache.Set(cacheKey, &fake.EngineWithErr{DefaultErr: err})
				}
			})
		}
		wg.Wait()
		time.Sleep(e.config.ConnectionTimeout)
	}
}

func (e *EngineCache) CheckNodeStatus(ctx context.Context) {
	logger := log.WithFunc("engine.factory.CheckNodeStatus")
	logger.Info(ctx, "check NodeStatus starts")
	defer logger.Info(ctx, "check NodeStatus ends")
	if e.sto == nil {
		logger.Warnf(ctx, "nodeStore is nil")
		return
	}
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		ch := e.sto.NodeStatusStream(ctx)

		for ns := range ch {
			if ns.Alive {
				// GetNode will call GetEngine, so GetNode updates the engine cache automatically
				if _, err := e.sto.GetNode(ctx, ns.Nodename); err != nil {
					logger.Warnf(ctx, "failed to get node %s: %s", ns.Nodename, err)
				}
			} else {
				// a node may have multiple engines, so we need check all key here
				e.keysToCheck.ForEach(func(k string, ep engineParams) bool {
					if ep.nodename == ns.Nodename {
						logger.Infof(ctx, "remove engine %+v from cache", ep.getCacheKey())
						RemoveEngineFromCache(ctx, ep.endpoint, ep.ca, ep.cert, ep.key)
					}
					return true
				})
			}
		}
	}
}

// GetEngineFromCache .
func GetEngineFromCache(_ context.Context, endpoint, ca, cert, key string) engine.API {
	return engineCache.Get(getEngineCacheKey(endpoint, ca, cert, key))
}

// RemoveEngineFromCache .
func RemoveEngineFromCache(ctx context.Context, endpoint, ca, cert, key string) {
	cacheKey := getEngineCacheKey(endpoint, ca, cert, key)
	log.WithFunc("engine.factory.RemoveEngineFromCache").Infof(ctx, "remove engine %+v from cache", cacheKey)
	engineCache.Delete(cacheKey)
}

func GetNodeEngine(ctx context.Context, config types.Config, node *types.Node) (client engine.API, err error) {
	if err := engineCache.sto.LoadNodeCert(ctx, node); err != nil {
		return nil, err
	}
	return GetEngine(ctx, config, node.Name, node.Endpoint, node.Ca, node.Cert, node.Key)
}

func GetWorkloadEngine(ctx context.Context, config types.Config, wrk *types.Workload) (client engine.API, err error) {
	node, err := engineCache.sto.GetNode(ctx, wrk.Nodename)
	if err != nil {
		return nil, err
	}
	return GetNodeEngine(ctx, config, node)
}

// GetEngine get engine with cache
func GetEngine(ctx context.Context, config types.Config, nodename, endpoint, ca, cert, key string) (client engine.API, err error) {
	logger := log.WithFunc("engine.factory.GetEngine")
	if client = GetEngineFromCache(ctx, endpoint, ca, cert, key); client != nil {
		return client, nil
	}

	defer func() {
		params := engineParams{
			nodename: nodename,
			endpoint: endpoint,
			ca:       ca,
			cert:     cert,
			key:      key,
		}
		cacheKey := params.getCacheKey()
		if err == nil {
			engineCache.Set(params, client)
			logger.Infof(ctx, "store engine %+v in cache", cacheKey)
		} else {
			engineCache.Set(params, &fake.EngineWithErr{DefaultErr: err})
			logger.Infof(ctx, "store fake engine %+v in cache", cacheKey)
		}
	}()

	return newEngine(ctx, config, nodename, endpoint, ca, cert, key)
}

type engineParams struct {
	nodename string
	endpoint string
	ca       string
	cert     string
	key      string
}

func (ep engineParams) getCacheKey() string {
	return getEngineCacheKey(ep.endpoint, ep.ca, ep.cert, ep.key)
}

func validateEngine(ctx context.Context, engine engine.API, timeout time.Duration) (err error) {
	utils.WithTimeout(ctx, timeout, func(ctx context.Context) {
		err = engine.Ping(ctx)
	})
	return err
}

func getEnginePrefix(endpoint string) (string, error) {
	for prefix := range engines {
		if strings.HasPrefix(endpoint, prefix) {
			return prefix, nil
		}
	}
	return "", errors.Wrapf(types.ErrInvaildNodeEndpoint, "endpoint invalid %+v", endpoint)
}

func getEngineCacheKey(endpoint, ca, cert, key string) string {
	return fmt.Sprintf("%+v-%+v", endpoint, utils.SHA256(fmt.Sprintf(":%+v:%+v:%+v", ca, cert, key))[:8])
}

// newEngine get engine
func newEngine(ctx context.Context, config types.Config, nodename, endpoint, ca, cert, key string) (client engine.API, err error) {
	prefix, err := getEnginePrefix(endpoint)
	if err != nil {
		return nil, err
	}
	e, ok := engines[prefix]
	if !ok {
		return nil, types.ErrInvaildEngineEndpoint
	}
	utils.WithTimeout(ctx, config.ConnectionTimeout, func(ctx context.Context) {
		client, err = e(ctx, config, nodename, endpoint, ca, cert, key)
	})
	if err != nil {
		return nil, err
	}
	if err = validateEngine(ctx, client, config.ConnectionTimeout); err != nil {
		log.WithFunc("engine.factory.newEngine").Errorf(ctx, err, "engine of %+v is unavailable", endpoint)
		return nil, err
	}
	return client, nil
}
