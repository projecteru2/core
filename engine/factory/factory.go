package factory

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/projecteru2/core/engine"
	"github.com/projecteru2/core/engine/docker"
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
	engineCache = utils.NewEngineCache(12*time.Hour, 10*time.Minute)
	keysToCheck = sync.Map{}
)

func getEngineCacheKey(endpoint, ca, cert, key string) string {
	return fmt.Sprintf("%v-%v", endpoint, utils.SHA256(fmt.Sprintf(":%v:%v:%v", ca, cert, key))[:8])
}

// EngineCacheChecker checks if the engine in cache is available
func EngineCacheChecker(ctx context.Context, timeout time.Duration) {
	log.Info("[EngineCacheChecker] starts")
	defer log.Info("[EngineCacheChecker] ends")
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		keysToRemove := []string{}
		keysToCheck.Range(func(key, _ interface{}) bool {
			cacheKey := key.(string)
			client := engineCache.Get(cacheKey)
			if client == nil {
				keysToRemove = append(keysToRemove, cacheKey)
				return true
			}
			if err := validateEngine(ctx, client, timeout); err != nil {
				log.Errorf(ctx, "[GetEngineFromCache] engine %v is unavailable, will be removed from cache, err: %v", cacheKey, err)
				keysToRemove = append(keysToRemove, cacheKey)
			}
			return true
		})
		for _, key := range keysToRemove {
			engineCache.Delete(key)
			keysToCheck.Delete(key)
		}
		time.Sleep(timeout)
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
	log.Infof(context.TODO(), "[RemoveEngineFromCache] remove engine %v from cache", cacheKey)
	engineCache.Delete(cacheKey)
}

// GetEngine get engine
func GetEngine(ctx context.Context, config types.Config, nodename, endpoint, ca, cert, key string) (client engine.API, err error) {
	if client = GetEngineFromCache(endpoint, ca, cert, key); client != nil {
		return client, nil
	}

	defer func() {
		if err == nil && client != nil {
			cacheKey := getEngineCacheKey(endpoint, ca, cert, key)
			engineCache.Set(cacheKey, client)
			keysToCheck.Store(cacheKey, struct{}{})
			log.Infof(ctx, "[GetEngine] store engine %v in cache", cacheKey)
		}
	}()

	prefix, err := getEnginePrefix(endpoint)
	if err != nil {
		return nil, err
	}
	e, ok := engines[prefix]
	if !ok {
		return nil, types.ErrNotSupport
	}
	if client, err = e(ctx, config, nodename, endpoint, ca, cert, key); err != nil {
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
