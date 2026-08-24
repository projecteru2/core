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
	TCPPrefixKey  = "tcp://"
	SockPrefixKey = "unix://"
	Type          = "docker"
)

type Engine struct {
	client dockerapi.APIClient
	config coretypes.Config
	ep     *enginetypes.Params
}

func (e *Engine) Info(ctx context.Context) (*enginetypes.Info, error) {
	r, err := e.client.Info(ctx)
	if err != nil {
		return nil, err
	}
	return &enginetypes.Info{Type: Type, ID: r.ID, NCPU: r.NCPU, MemTotal: r.MemTotal}, nil
}

func (e *Engine) Ping(ctx context.Context) error {
	_, err := e.client.Ping(ctx)
	return err
}

func (e *Engine) CloseConn() error {
	return e.client.Close()
}

func (e *Engine) GetParams() *enginetypes.Params {
	return e.ep
}

// MakeClient builds a docker engine for endpoint.
func MakeClient(ctx context.Context, config coretypes.Config, nodename, endpoint, ca, cert, key string) (engine.API, error) {
	var client *http.Client
	var err error
	logger := log.WithFunc("engine.docker.MakeClient")
	if strings.HasPrefix(endpoint, "unix://") {
		client = utils.GetUnixSockClient()
	} else {
		client, err = utils.GetHTTPSClient(ctx, config.CertPath, nodename, ca, cert, key)
		if err != nil {
			logger.Errorf(ctx, err, "get https client for %s %s", nodename, endpoint)
			return nil, err
		}
	}

	logger.Debugf(ctx, "create docker client for %s, %s", endpoint, config.Docker.APIVersion)
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

func makeDockerClient(_ context.Context, config coretypes.Config, client *http.Client, endpoint string) (*Engine, error) {
	// the docker client rewrites Transport on the *http.Client it is given, so the shared one is copied
	own := *client
	cli, err := dockerapi.NewClientWithOpts(
		dockerapi.WithHost(endpoint),
		dockerapi.WithVersion(config.Docker.APIVersion),
		dockerapi.WithHTTPClient(&own),
	)
	if err != nil {
		return nil, err
	}
	return &Engine{client: cli, config: config}, nil
}
