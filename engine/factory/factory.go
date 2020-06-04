package factory

import (
	"context"
	"fmt"
	"strings"

	"github.com/projecteru2/core/engine"
	"github.com/projecteru2/core/engine/docker"
	"github.com/projecteru2/core/engine/mocks/fakeengine"
	"github.com/projecteru2/core/engine/systemd"
	"github.com/projecteru2/core/engine/virt"
	"github.com/projecteru2/core/types"
)

type factory func(ctx context.Context, config types.Config, nodename, endpoint, ca, cert, key string) (engine.API, error)

var engines = map[string]factory{
	docker.TCPPrefixKey:  docker.MakeClient,
	docker.SockPrefixKey: docker.MakeClient,
	virt.HTTPPrefixKey:   virt.MakeClient,
	virt.GRPCPrefixKey:   virt.MakeClient,
	systemd.SSHPrefixKey: systemd.MakeClient,
	fakeengine.PrefixKey: fakeengine.MakeClient,
}

// GetEngine get engine
func GetEngine(ctx context.Context, config types.Config, nodename, endpoint, ca, cert, key string) (engine.API, error) {
	prefix, err := getEnginePrefix(endpoint)
	if err != nil {
		return nil, err
	}
	e, ok := engines[prefix]
	if !ok {
		return nil, types.ErrNotSupport
	}
	api, err := e(ctx, config, nodename, endpoint, ca, cert, key)
	return engine.WithGlobalTimeout(api, config.GlobalTimeout), err
}

func getEnginePrefix(endpoint string) (string, error) {
	for prefix := range engines {
		if strings.HasPrefix(endpoint, prefix) {
			return prefix, nil
		}
	}
	return "", types.NewDetailedErr(types.ErrNodeFormat, fmt.Sprintf("endpoint invalid %v", endpoint))
}
