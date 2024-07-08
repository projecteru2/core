package systemd

import (
	"context"

	"github.com/projecteru2/core/engine"
	"github.com/projecteru2/core/engine/docker"
	enginetypes "github.com/projecteru2/core/engine/types"
	coretypes "github.com/projecteru2/core/types"
)

// TCPPrefix is engine endpoint prefix
const TCPPrefix = "systemd://"

// Engine is engine for systemd
type Engine struct {
	engine.API
	config coretypes.Config
	ep     *enginetypes.Params
}

// MakeClient make systemd cli
func MakeClient(ctx context.Context, config coretypes.Config, nodename, endpoint, ca, cert, key string) (engine.API, error) {
	api, err := docker.MakeClient(ctx, config, nodename, endpoint, ca, cert, key)
	if err != nil {
		return nil, err
	}
	ep := &enginetypes.Params{
		Nodename: nodename,
		Endpoint: endpoint,
		CA:       ca,
		Cert:     cert,
		Key:      key,
	}
	return &Engine{
		API:    api,
		config: config,
		ep:     ep,
	}, nil
}

func (e *Engine) GetParams() *enginetypes.Params {
	return e.ep
}
