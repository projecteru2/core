package systemd

import (
	"context"

	"github.com/projecteru2/core/engine"
	"github.com/projecteru2/core/engine/docker"
	coretypes "github.com/projecteru2/core/types"
)

// SystemdPrefixKey is engine endpoint prefix
const SystemdPrefixKey = "systemd://"

type systemdEngine struct {
	engine.API
	config coretypes.Config
}

// MakeClient make docker cli
func MakeClient(ctx context.Context, config coretypes.Config, nodename, endpoint, ca, cert, key string) (engine.API, error) {
	var (
		api engine.API
		err error
	)
	if api, err = docker.MakeClient(ctx, config, nodename, endpoint, ca, cert, key); err != nil {
		return nil, err
	}
	return &systemdEngine{
		API:    api,
		config: config,
	}, nil
}
