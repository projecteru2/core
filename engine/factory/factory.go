package factory

import (
	"context"
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
	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/engine/virt"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/metrics"
	"github.com/projecteru2/core/store"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

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

type factory func(ctx context.Context, config types.Config, nodename, endpoint, ca, cert, key string) (engine.API, error)

// EngineCache holds one engine client per node endpoint and keeps it alive.
type EngineCache struct {
	cache  *haxmap.Map[string, engine.API]
	pool   *ants.PoolWithFunc
	config types.Config
	stor   store.Store
}

// NewEngineCache builds an empty engine cache.
func NewEngineCache(config types.Config, stor store.Store) *EngineCache {
	pool, _ := utils.NewPool(config.MaxConcurrency)
	return &EngineCache{
		cache:  haxmap.New[string, engine.API](),
		pool:   pool,
		config: config,
		stor:   stor,
	}
}

func (e *EngineCache) Get(key string) engine.API {
	// haxmap unlinks lazily and still returns the old value for a deleted key
	if api, ok := e.cache.Get(key); ok {
		return api
	}
	return nil
}

func (e *EngineCache) Set(key string, client engine.API) {
	e.cache.Set(key, client)
}

func (e *EngineCache) Delete(key string) {
	e.cache.Del(key)
}

func (e *EngineCache) checkAlive(ctx context.Context) {
	logger := log.WithFunc("engine.factory.checkAlive")
	logger.Info(ctx, "check alive starts")
	defer logger.Info(ctx, "check alive ends")
	defer e.pool.Release()
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		wg := &sync.WaitGroup{}
		e.cache.ForEach(func(_ string, v engine.API) bool {
			wg.Add(1)
			params := v.GetParams()
			_ = e.pool.Invoke(func() {
				defer wg.Done()
				cacheKey := params.CacheKey()
				client := e.Get(cacheKey)
				if client == nil {
					e.Delete(cacheKey)
					return
				}
				if _, ok := client.(*fake.EngineWithErr); ok {
					if newClient, err := newEngine(ctx, e.config, params); err != nil {
						logger.Errorf(ctx, err, "engine %+v is still unavailable", cacheKey)
						e.Set(cacheKey, &fake.EngineWithErr{DefaultErr: err, EP: params})
						e.checkOneNodeStatus(ctx, params)
					} else {
						e.Set(cacheKey, newClient)
					}
					return
				}
				if err := validateEngine(ctx, client, e.config.ConnectionTimeout); err != nil {
					logger.Errorf(ctx, err, "engine %+v is unavailable, will be replaced and removed", cacheKey)
					e.Set(cacheKey, &fake.EngineWithErr{DefaultErr: err, EP: params})
				}
				logger.Debugf(ctx, "engine %+v is available", cacheKey)
			})
			return true
		})
		wg.Wait()
		time.Sleep(e.config.ConnectionTimeout)
	}
}

func (e *EngineCache) checkNodeStatus(ctx context.Context) {
	logger := log.WithFunc("engine.factory.checkNodeStatus")
	logger.Info(ctx, "check node status starts")
	defer logger.Info(ctx, "check node status ends")
	if e.stor == nil {
		logger.Warn(ctx, "node store is nil")
		return
	}
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		ch := e.stor.NodeStatusStream(ctx)

		// alive nodes are re-cached by NodeStatusStream's own GetNode call
		for ns := range ch {
			if errors.Is(ns.Error, types.ErrInvaildCount) {
				logger.Infof(ctx, "remove metrics for invalid node %s", ns.Nodename)
				metrics.Client.RemoveInvalidNodes(ns.Nodename)
			}

			if !ns.Alive {
				// one node may back several engines
				e.cache.ForEach(func(_ string, v engine.API) bool {
					ep := v.GetParams()
					if ep.Nodename == ns.Nodename {
						logger.Infof(ctx, "remove engine %+v from cache", ep.CacheKey())
						RemoveEngineFromCache(ctx, ep.Endpoint, ep.CA, ep.Cert, ep.Key)
					}
					return true
				})
			}
		}
	}
}

func (e *EngineCache) checkOneNodeStatus(ctx context.Context, params *enginetypes.Params) {
	if e.stor == nil {
		return
	}
	logger := log.WithFunc("engine.factory.checkOneNodeStatus")
	nodename := params.Nodename
	cacheKey := params.CacheKey()
	ns, err := e.stor.GetNodeStatus(ctx, nodename)
	if err != nil && !errors.Is(err, types.ErrInvaildCount) {
		logger.Errorf(ctx, err, "get status of node %s", nodename)
		return
	}
	if err == nil && ns.Alive {
		return
	}
	logger.Warnf(ctx, "node %s is offline, the cache will be removed", nodename)
	e.Delete(cacheKey)
}

// InitEngineCache builds the global engine cache and starts its checkers.
func InitEngineCache(ctx context.Context, config types.Config, stor store.Store) {
	engineCache = NewEngineCache(config, stor)
	// warm the cache; the return values are irrelevant
	if stor != nil {
		_, _ = engineCache.stor.GetNodesByPod(ctx, &types.NodeFilter{
			All: true,
		})
	}
	go engineCache.checkAlive(ctx)
	go engineCache.checkNodeStatus(ctx)
}

// GetEngineFromCache returns the cached engine for an endpoint, or nil.
func GetEngineFromCache(_ context.Context, endpoint, ca, cert, key string) engine.API {
	return engineCache.Get(getEngineCacheKey(endpoint, ca, cert, key))
}

// RemoveEngineFromCache drops the cached engine for an endpoint.
func RemoveEngineFromCache(ctx context.Context, endpoint, ca, cert, key string) {
	cacheKey := getEngineCacheKey(endpoint, ca, cert, key)
	log.WithFunc("engine.factory.RemoveEngineFromCache").Infof(ctx, "remove engine %+v from cache", cacheKey)
	engineCache.Delete(cacheKey)
}

// GetEngine returns the cached engine for an endpoint, building one if absent.
func GetEngine(ctx context.Context, config types.Config, nodename, endpoint, ca, cert, key string) (client engine.API, err error) {
	logger := log.WithFunc("engine.factory.GetEngine")
	if client = GetEngineFromCache(ctx, endpoint, ca, cert, key); client != nil {
		return client, nil
	}

	params := &enginetypes.Params{
		Nodename: nodename,
		Endpoint: endpoint,
		CA:       ca,
		Cert:     cert,
		Key:      key,
	}
	defer func() {
		cacheKey := params.CacheKey()
		if err == nil {
			engineCache.Set(cacheKey, client)
			logger.Infof(ctx, "store engine %+v in cache", cacheKey)
		} else {
			engineCache.Set(cacheKey, &fake.EngineWithErr{DefaultErr: err, EP: params})
			logger.Infof(ctx, "store fake engine %+v in cache", cacheKey)
		}
	}()

	return newEngine(ctx, config, params)
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
	p := enginetypes.Params{
		Endpoint: endpoint,
		CA:       ca,
		Cert:     cert,
		Key:      key,
	}
	return p.CacheKey()
}

func newEngine(ctx context.Context, config types.Config, params *enginetypes.Params) (client engine.API, err error) {
	prefix, err := getEnginePrefix(params.Endpoint)
	if err != nil {
		return nil, err
	}
	e, ok := engines[prefix]
	if !ok {
		return nil, types.ErrInvaildEngineEndpoint
	}
	utils.WithTimeout(ctx, config.ConnectionTimeout, func(ctx context.Context) {
		client, err = e(ctx, config, params.Nodename, params.Endpoint, params.CA, params.Cert, params.Key)
	})
	if err != nil {
		return nil, err
	}
	if err = validateEngine(ctx, client, config.ConnectionTimeout); err != nil {
		log.WithFunc("engine.factory.newEngine").Errorf(ctx, err, "engine of %+v is unavailable", params.Endpoint)
		return nil, err
	}
	return client, nil
}
