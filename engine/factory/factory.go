package factory

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/cockroachdb/errors"
	"github.com/cornelk/hashmap"
	"github.com/panjf2000/ants/v2"

	"github.com/projecteru2/core/engine"
	"github.com/projecteru2/core/engine/docker"
	"github.com/projecteru2/core/engine/fake"
	"github.com/projecteru2/core/engine/mocks/fakeengine"
	"github.com/projecteru2/core/engine/systemd"
	"github.com/projecteru2/core/engine/virt"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

type factory func(ctx context.Context, config types.Config, nodename, endpoint, ca, cert, key string) (engine.API, error)

var (
	engines = map[string]factory{
		docker.TCPPrefixKey:  docker.MakeClient,
		docker.SockPrefixKey: docker.MakeClient,
		virt.HTTPPrefixKey:   virt.MakeClient,
		virt.GRPCPrefixKey:   virt.MakeClient,
		systemd.TCPPrefix:    systemd.MakeClient,
		fakeengine.PrefixKey: fakeengine.MakeClient,
	}
	engineCache *EngineCache
)

// EngineCache .
type EngineCache struct {
	cache       *utils.EngineCache
	keysToCheck *hashmap.Map[uintptr, engineParams]
	pool        *ants.PoolWithFunc
	config      types.Config
}

// NewEngineCache .
func NewEngineCache(config types.Config) *EngineCache {
	pool, _ := utils.NewPool(config.MaxConcurrency)
	return &EngineCache{
		cache:       utils.NewEngineCache(12*time.Hour, 10*time.Minute),
		keysToCheck: hashmap.New[uintptr, engineParams](),
		pool:        pool,
		config:      config,
	}
}

// InitEngineCache init engine cache and start engine cache checker
func InitEngineCache(ctx context.Context, config types.Config) {
	engineCache = NewEngineCache(config)
	go engineCache.CheckAlive(ctx)
}

// Get .
func (e *EngineCache) Get(key string) engine.API {
	return e.cache.Get(key)
}

// Set .
func (e *EngineCache) Set(params engineParams, client engine.API) {
	e.cache.Set(params.getCacheKey(), client)
	e.keysToCheck.Set(uintptr(unsafe.Pointer(&params)), params)
}

// Delete .
func (e *EngineCache) Delete(key string) {
	e.cache.Delete(key)
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
			e.keysToCheck.Range(func(k uintptr, v engineParams) bool {
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
					e.keysToCheck.Del(uintptr(unsafe.Pointer(&params)))
					return
				}
				if _, ok := client.(*fake.EngineWithErr); ok {
					if newClient, err := newEngine(ctx, e.config, params.nodename, params.endpoint, params.ca, params.key, params.cert); err != nil {
						logger.Errorf(ctx, err, "engine %+v is still unavailable", cacheKey)
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
