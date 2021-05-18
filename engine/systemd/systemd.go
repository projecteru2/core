package systemd

import (
	"context"

	"github.com/projecteru2/core/engine"
	"github.com/projecteru2/core/engine/docker"
	coretypes "github.com/projecteru2/core/types"
)

// TCPPrefix is engine endpoint prefix
const TCPPrefix = "systemd://"

// Engine is engine for systemd
type Engine struct {
	engine.API
	config coretypes.Config
}

// MakeClient make systemd cli
func MakeClient(ctx context.Context, config coretypes.Config, nodename, endpoint, ca, cert, key string) (engine.API, error) {
	api, err := docker.MakeClient(ctx, config, nodename, endpoint, ca, cert, key)
	if err != nil {
		return nil, err
	}
	return &Engine{
		API:    api,
		config: config,
	}, nil
}
