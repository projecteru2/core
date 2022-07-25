package docker

import (
	"context"
	"net/http"
	"strings"

	"github.com/projecteru2/core/engine"
	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/log"
	coretypes "github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"

	dockerapi "github.com/docker/docker/client"
)

const (
	// TCPPrefixKey indicate tcp prefix
	TCPPrefixKey = "tcp://"
	// SockPrefixKey indicate sock prefix
	SockPrefixKey = "unix://"
)

// Engine is engine for docker
type Engine struct {
	client dockerapi.APIClient
	config coretypes.Config
}

// MakeClient make docker cli
func MakeClient(ctx context.Context, config coretypes.Config, nodename, endpoint, ca, cert, key string) (engine.API, error) {
	var client *http.Client
	var err error
	if strings.HasPrefix(endpoint, "unix://") {
		client = utils.GetUnixSockClient()
	} else {
		client, err = utils.GetHTTPSClient(ctx, config.CertPath, nodename, ca, cert, key)
		if err != nil {
			log.Errorf(ctx, "[MakeClient] GetHTTPSClient for %s %s error: %v", nodename, endpoint, err)
			return nil, err
		}
	}

	log.Debugf(ctx, "[MakeDockerEngine] Create new http.Client for %s, %s", endpoint, config.Docker.APIVersion)
	return makeDockerClient(ctx, config, client, endpoint)
}

// Info show node info
// 2 seconds timeout
// used to be 5, but client won't wait that long
func (e *Engine) Info(ctx context.Context) (*enginetypes.Info, error) {
	r, err := e.client.Info(ctx)
	if err != nil {
		return nil, err
	}
	return &enginetypes.Info{ID: r.ID, NCPU: r.NCPU, MemTotal: r.MemTotal}, nil
}

// ResourceValidate validate resource usage
func (e *Engine) ResourceValidate(ctx context.Context, cpu float64, cpumap map[string]int64, memory, storage int64) error {
	// TODO list all workloads, calcuate resource
	return nil
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
