package docker

import (
	"context"
	"net/http"
	"strings"

	dockerapi "github.com/docker/docker/client"

	"github.com/projecteru2/core/engine"
	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/log"
	coretypes "github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

const (
	// TCPPrefixKey indicate tcp prefix
	TCPPrefixKey = "tcp://"
	// SockPrefixKey indicate sock prefix
	SockPrefixKey = "unix://"
	// Type indicate docker
	Type = "docker"
)

// Engine is engine for docker
type Engine struct {
	client dockerapi.APIClient
	config coretypes.Config
	ep     *enginetypes.Params
}

// MakeClient make docker cli
func MakeClient(ctx context.Context, config coretypes.Config, nodename, endpoint, ca, cert, key string) (engine.API, error) {
	var client *http.Client
	var err error
	logger := log.WithFunc("engine.docker.MakeClient")
	if strings.HasPrefix(endpoint, "unix://") {
		client = utils.GetUnixSockClient()
	} else {
		client, err = utils.GetHTTPSClient(ctx, config.CertPath, nodename, ca, cert, key)
		if err != nil {
			logger.Errorf(ctx, err, "GetHTTPSClient for %s %s", nodename, endpoint)
			return nil, err
		}
	}

	logger.Debugf(ctx, "Create new http.Client for %s, %s", endpoint, config.Docker.APIVersion)
	e, err := makeDockerClient(ctx, config, client, endpoint)
	if err != nil {
		return nil, err
	}
	e.ep = &enginetypes.Params{
		Nodename: nodename,
		Endpoint: endpoint,
		CA:       ca,
		Cert:     cert,
		Key:      key,
	}
	return e, nil
}

// Info show node info
// 2 seconds timeout
// used to be 5, but client won't wait that long
func (e *Engine) Info(ctx context.Context) (*enginetypes.Info, error) {
	r, err := e.client.Info(ctx)
	if err != nil {
		return nil, err
	}
	return &enginetypes.Info{Type: Type, ID: r.ID, NCPU: r.NCPU, MemTotal: r.MemTotal}, nil
}

func (e *Engine) GetParams() *enginetypes.Params {
	return e.ep
}

// Ping test connection
func (e *Engine) Ping(ctx context.Context) error {
	_, err := e.client.Ping(ctx)
	return err
}

// CloseConn close connection
func (e *Engine) CloseConn() error {
	return e.client.Close()
}
