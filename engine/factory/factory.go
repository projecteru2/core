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

// GetEngine get engine
func GetEngine(ctx context.Context, config types.Config, nodename, endpoint, ca, cert, key string) (client engine.API, err error) {
	if client = engineCache.Get(endpoint); client != nil {
		return
	}

	defer func() {
		if err == nil && client != nil {
			engineCache.Set(endpoint, client)
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
	return e(ctx, config, nodename, endpoint, ca, cert, key)
}

func getEnginePrefix(endpoint string) (string, error) {
	for prefix := range engines {
		if strings.HasPrefix(endpoint, prefix) {
			return prefix, nil
		}
	}
	return "", types.NewDetailedErr(types.ErrNodeFormat, fmt.Sprintf("endpoint invalid %v", endpoint))
}
