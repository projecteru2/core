package factory

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/panjf2000/ants/v2"

	"github.com/projecteru2/core/engine"
	"github.com/projecteru2/core/engine/containerd"
	"github.com/projecteru2/core/engine/fake"
	"github.com/projecteru2/core/engine/mocks/fakeengine"
	"github.com/projecteru2/core/engine/process"
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
		virt.GRPCPrefixKey:   virt.MakeClient,
		containerd.Prefix:    containerd.MakeClient,
		process.Prefix:       process.MakeClient,
		fakeengine.PrefixKey: fakeengine.MakeClient,
	}

	engineCache *EngineCache
)

type factory func(ctx context.Context, config types.Config, nodename, endpoint, ca, cert, key string) (engine.API, error)

// EngineCache holds one engine client per node endpoint and keeps it alive.
type EngineCache struct {
	cache  sync.Map
	pool   *ants.PoolWithFunc
	config types.Config
	stor   store.Store
}

// NewEngineCache builds an empty engine cache.
func NewEngineCache(config types.Config, stor store.Store) *EngineCache {
	pool, _ := utils.NewPool(config.MaxConcurrency)
	return &EngineCache{
		pool:   pool,
		config: config,
		stor:   stor,
	}
}

func (e *EngineCache) Get(key string) engine.API {
	if api, ok := e.cache.Load(key); ok {
		return api.(engine.API)
	}
	return nil
}

func (e *EngineCache) Set(key string, client engine.API) {
	e.cache.Store(key, client)
}

func (e *EngineCache) Delete(key string) {
	if api, ok := e.cache.Load(key); ok {
		closeEngine(api.(engine.API))
	}
	e.cache.Delete(key)
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
		e.cache.Range(func(_, v any) bool {
			wg.Add(1)
			params := v.(engine.API).GetParams()
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
					} else {
						e.Set(cacheKey, newClient)
					}
					return
				}
				if err := validateEngine(ctx, client, e.config.ConnectionTimeout); err != nil {
					logger.Errorf(ctx, err, "engine %+v is unavailable, will be replaced and removed", cacheKey)
					closeEngine(client)
					e.Set(cacheKey, &fake.EngineWithErr{DefaultErr: err, EP: params})
					return
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
				e.cache.Range(func(_, v any) bool {
					ep := v.(engine.API).GetParams()
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

// InitEngineCache builds the global engine cache and starts its checkers.
func InitEngineCache(ctx context.Context, config types.Config, stor store.Store) {
	engineCache = NewEngineCache(config, stor)
	if stor != nil {
		_, _ = engineCache.stor.GetNodesByPod(ctx, &types.NodeFilter{
			All: true,
		}, false)
	}
	go engineCache.checkAlive(ctx)
	go engineCache.checkNodeStatus(ctx)
}

// GetEngineFromCache returns the cached engine for an endpoint, or nil.
func GetEngineFromCache(_ context.Context, endpoint, ca, cert, key string) engine.API {
	return engineCache.Get(enginetypes.EndpointCacheKey(endpoint, ca, cert, key))
}

// RemoveEngineFromCache drops the cached engine for an endpoint.
func RemoveEngineFromCache(ctx context.Context, endpoint, ca, cert, key string) {
	cacheKey := enginetypes.EndpointCacheKey(endpoint, ca, cert, key)
	log.WithFunc("engine.factory.RemoveEngineFromCache").Infof(ctx, "remove engine %+v from cache", cacheKey)
	engineCache.Delete(cacheKey)
}

// GetEngine returns the cached engine for an endpoint, building one if absent.
func GetEngine(ctx context.Context, config types.Config, nodename, endpoint, ca, cert, key string) (client engine.API, err error) {
	logger := log.WithFunc("engine.factory.GetEngine")
	if client = GetEngineFromCache(ctx, endpoint, ca, cert, key); client != nil {
		return client, nil
	}

	params := enginetypes.NewParams(nodename, endpoint, ca, cert, key)
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

// closeEngine releases the connection an engine owns; a fake engine holds none.
func closeEngine(api engine.API) {
	if _, ok := api.(*fake.EngineWithErr); ok {
		return
	}
	_ = api.CloseConn()
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
		closeEngine(client)
		return nil, err
	}
	return client, nil
}
