package factory

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
	"unsafe"

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

func getEngineCacheKey(endpoint, ca, cert, key string) string {
	return fmt.Sprintf("%v-%v", endpoint, utils.SHA256(fmt.Sprintf(":%v:%v:%v", ca, cert, key))[:8])
}

type engineParams struct {
	endpoint string
	ca       string
	cert     string
	key      string
}

func (ep engineParams) getCacheKey() string {
	return getEngineCacheKey(ep.endpoint, ep.ca, ep.cert, ep.key)
}

// EngineCache .
type EngineCache struct {
	cache       *utils.EngineCache
	keysToCheck hashmap.HashMap
	pool        *ants.PoolWithFunc
	config      types.Config
}

// NewEngineCache .
func NewEngineCache(config types.Config) *EngineCache {
	pool, _ := utils.NewPool(config.MaxConcurrency)
	return &EngineCache{
		cache:       utils.NewEngineCache(12*time.Hour, 10*time.Minute),
		keysToCheck: hashmap.HashMap{},
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
	log.Info("[EngineCache] starts")
	defer log.Info("[EngineCache] ends")
	defer e.pool.Release()
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		paramsChan := make(chan engineParams)
		go func() {
			for kv := range e.keysToCheck.Iter() {
				paramsChan <- kv.Value.(engineParams)
			}
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
				if _, ok := client.(*fake.Engine); ok {
					if newClient, err := newEngine(ctx, e.config, utils.RandomString(8), params.endpoint, params.ca, params.key, params.cert); err != nil {
						log.Errorf(ctx, "[EngineCache] engine %v is still unavailable, err: %v", cacheKey, err)
					} else {
						e.cache.Set(cacheKey, newClient)
					}
					return
				}
				if err := validateEngine(ctx, client, e.config.ConnectionTimeout); err != nil {
					log.Errorf(ctx, "[EngineCache] engine %v is unavailable, will be replaced with a fake engine, err: %v", cacheKey, err)
					e.cache.Set(cacheKey, &fake.Engine{DefaultErr: err})
				}
			})
		}
		wg.Wait()
		time.Sleep(e.config.ConnectionTimeout)
	}
}

func validateEngine(ctx context.Context, engine engine.API, timeout time.Duration) (err error) {
	utils.WithTimeout(ctx, timeout, func(ctx context.Context) {
		err = engine.Ping(ctx)
	})
	return err
}

// GetEngineFromCache .
func GetEngineFromCache(endpoint, ca, cert, key string) engine.API {
	return engineCache.Get(getEngineCacheKey(endpoint, ca, cert, key))
}

// RemoveEngineFromCache .
func RemoveEngineFromCache(endpoint, ca, cert, key string) {
	cacheKey := getEngineCacheKey(endpoint, ca, cert, key)
	log.Infof(nil, "[RemoveEngineFromCache] remove engine %v from cache", cacheKey) // nolint
	engineCache.Delete(cacheKey)
}

// GetEngine get engine with cache
func GetEngine(ctx context.Context, config types.Config, nodename, endpoint, ca, cert, key string) (client engine.API, err error) {
	if client = GetEngineFromCache(endpoint, ca, cert, key); client != nil {
		return client, nil
	}

	defer func() {
		params := engineParams{
			endpoint: endpoint,
			ca:       ca,
			cert:     cert,
			key:      key,
		}
		cacheKey := params.getCacheKey()
		if err == nil {
			engineCache.Set(params, client)
			log.Infof(ctx, "[GetEngine] store engine %v in cache", cacheKey)
		} else {
			engineCache.Set(params, &fake.Engine{DefaultErr: err})
			log.Infof(ctx, "[GetEngine] store fake engine %v in cache", cacheKey)
		}
	}()

	return newEngine(ctx, config, nodename, endpoint, ca, cert, key)
}

// newEngine get engine
func newEngine(ctx context.Context, config types.Config, nodename, endpoint, ca, cert, key string) (client engine.API, err error) {
	prefix, err := getEnginePrefix(endpoint)
	if err != nil {
		return nil, err
	}
	e, ok := engines[prefix]
	if !ok {
		return nil, types.ErrNotSupport
	}
	utils.WithTimeout(ctx, config.ConnectionTimeout, func(ctx context.Context) {
		client, err = e(ctx, config, nodename, endpoint, ca, cert, key)
	})
	if err != nil {
		return nil, err
	}
	if err = validateEngine(ctx, client, config.ConnectionTimeout); err != nil {
		log.Errorf(ctx, "[GetEngine] engine of %v is unavailable, err: %v", endpoint, err)
		return nil, err
	}
	return client, nil
}

func getEnginePrefix(endpoint string) (string, error) {
	for prefix := range engines {
		if strings.HasPrefix(endpoint, prefix) {
			return prefix, nil
		}
	}
	return "", types.NewDetailedErr(types.ErrNodeFormat, fmt.Sprintf("endpoint invalid %v", endpoint))
}
