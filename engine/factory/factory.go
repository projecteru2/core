package factory

import (
	"context"
	"fmt"
	"strings"
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
)

func getEngineCacheKey(endpoint, ca, cert, key string) string {
	return utils.SHA256(fmt.Sprintf("%v:%v:%v:%v", endpoint, ca, cert, key))
}

func validateEngine(ctx context.Context, engine engine.API, timeout time.Duration) (err error) {
	utils.WithTimeout(ctx, timeout, func(ctx context.Context) {
		_, err = engine.Info(ctx)
	})
	return err
}

// GetEngineFromCache .
func GetEngineFromCache(ctx context.Context, config types.Config, endpoint, ca, cert, key string) engine.API {
	client := engineCache.Get(getEngineCacheKey(endpoint, ca, cert, key))
	if client == nil {
		return nil
	}
	if err := validateEngine(ctx, client, config.ConnectionTimeout); err != nil {
		log.Errorf(ctx, "[GetEngineFromCache] engine of %v is unavailable, will be removed from cache, err: %v", endpoint, err)
		RemoveEngineFromCache(endpoint, ca, cert, key)
		return nil
	}
	return client
}

// RemoveEngineFromCache .
func RemoveEngineFromCache(endpoint, ca, cert, key string) {
	cacheKey := getEngineCacheKey(endpoint, ca, cert, key)
	log.Debugf(context.TODO(), "[RemoveEngineFromCache] remove %v, key %v", endpoint, cacheKey)
	engineCache.Delete(cacheKey)
}

// GetEngine get engine
func GetEngine(ctx context.Context, config types.Config, nodename, endpoint, ca, cert, key string) (client engine.API, err error) {
	if client = GetEngineFromCache(ctx, config, endpoint, ca, cert, key); client != nil {
		return
	}

	defer func() {
		if err == nil && client != nil {
			cacheKey := getEngineCacheKey(endpoint, ca, cert, key)
			log.Debugf(ctx, "[GetEngine] store engine of %v in cache, key: %v", endpoint, cacheKey)
			engineCache.Set(cacheKey, client)
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
