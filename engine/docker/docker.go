package docker

import (
	"context"
	"net/http"
	"os"

	dockerapi "github.com/docker/docker/client"
	"github.com/docker/go-connections/tlsconfig"
	enginetypes "github.com/projecteru2/core/engine/types"
	coretypes "github.com/projecteru2/core/types"
	log "github.com/sirupsen/logrus"
)

// Engine is engine for docker
type Engine struct {
	client dockerapi.APIClient
	config coretypes.Config
}

// MakeRawClient make raw docker cli
func MakeRawClient(config coretypes.Config, client *http.Client, endpoint, apiversion string) (*Engine, error) {
	cli, err := dockerapi.NewClient(endpoint, apiversion, client, nil)
	if err != nil {
		return nil, err
	}
	return &Engine{cli, config}, nil
}

// MakeRawClientWithTLS make raw docker cli with TLS
func MakeRawClientWithTLS(config coretypes.Config, ca, cert, key *os.File, endpoint, apiversion string) (*Engine, error) {
	var client *http.Client
	options := tlsconfig.Options{
		CAFile:             ca.Name(),
		CertFile:           cert.Name(),
		KeyFile:            key.Name(),
		InsecureSkipVerify: true,
	}
	defer os.Remove(ca.Name())
	defer os.Remove(cert.Name())
	defer os.Remove(key.Name())
	tlsc, err := tlsconfig.Client(options)
	if err != nil {
		return nil, err
	}
	client = &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tlsc,
		},
	}
	log.Debugf("[MakeRawClientWithTLS] Create new http.Client for %s, %s", endpoint, apiversion)
	return MakeRawClient(config, client, endpoint, apiversion)
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
