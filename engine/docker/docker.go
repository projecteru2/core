package docker

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	dockerapi "github.com/docker/docker/client"
	"github.com/docker/go-connections/tlsconfig"
	"github.com/projecteru2/core/engine"
	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/log"
	coretypes "github.com/projecteru2/core/types"
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
	if config.CertPath != "" && ca != "" && cert != "" && key != "" { // nolint
		caFile, err := ioutil.TempFile(config.CertPath, fmt.Sprintf("ca-%s", nodename))
		if err != nil {
			return nil, err
		}
		certFile, err := ioutil.TempFile(config.CertPath, fmt.Sprintf("cert-%s", nodename))
		if err != nil {
			return nil, err
		}
		keyFile, err := ioutil.TempFile(config.CertPath, fmt.Sprintf("key-%s", nodename))
		if err != nil {
			return nil, err
		}
		if err = dumpFromString(caFile, certFile, keyFile, ca, cert, key); err != nil {
			return nil, err
		}
		options := tlsconfig.Options{
			CAFile:             caFile.Name(),
			CertFile:           certFile.Name(),
			KeyFile:            keyFile.Name(),
			InsecureSkipVerify: true,
		}
		defer os.Remove(caFile.Name())
		defer os.Remove(certFile.Name())
		defer os.Remove(keyFile.Name())
		tlsc, err := tlsconfig.Client(options)
		if err != nil {
			return nil, err
		}
		client = &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: tlsc,
			},
		}
	}

	log.Debugf("[MakeDockerEngine] Create new http.Client for %s, %s", endpoint, config.Docker.APIVersion)
	return makeRawClient(ctx, config, client, endpoint)
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
