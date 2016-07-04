package utils

import (
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	engineapi "github.com/docker/engine-api/client"
	"github.com/docker/go-connections/tlsconfig"
	"gitlab.ricebook.net/platform/core/types"
	"golang.org/x/net/context"
)

// cache connections
// otherwise they'll leak
type cache struct {
	sync.Mutex
	clients map[string]*engineapi.Client
}

func (c cache) set(host string, client *engineapi.Client) {
	c.Lock()
	defer c.Unlock()

	c.clients[host] = client
}

func (c cache) get(host string) *engineapi.Client {
	c.Lock()
	defer c.Unlock()
	return c.clients[host]
}

var _cache = cache{clients: make(map[string]*engineapi.Client)}

func MakeDockerClient(endpoint string, config types.Config, force bool) (*engineapi.Client, error) {
	if !strings.HasPrefix(endpoint, "tcp://") {
		endpoint = "tcp://" + endpoint
	}

	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}

	host, _, err := net.SplitHostPort(u.Host)
	if err != nil {
		return nil, err
	}

	// try get client, if nil, create a new one
	client := _cache.get(host)
	if client == nil || force {
		// if no cert path is set
		// then just use normal http client without tls
		var cli *http.Client
		if config.Docker.CertPath != "" {
			dockerCertPath := filepath.Join(config.Docker.CertPath, host)
			options := tlsconfig.Options{
				CAFile:             filepath.Join(dockerCertPath, "ca.pem"),
				CertFile:           filepath.Join(dockerCertPath, "cert.pem"),
				KeyFile:            filepath.Join(dockerCertPath, "key.pem"),
				InsecureSkipVerify: false,
			}
			tlsc, err := tlsconfig.Client(options)
			if err != nil {
				return nil, err
			}

			cli = &http.Client{
				Transport: &http.Transport{
					TLSClientConfig: tlsc,
				},
			}
		}

		log.Debugf("Create new http.Client for %q", endpoint)
		client, err = engineapi.NewClient(endpoint, config.Docker.APIVersion, cli, nil)
		if err != nil {
			return nil, err
		}

		_cache.set(host, client)
	}

	// timeout in 5 seconds
	// timeout means node is not available
	ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)
	_, err = client.Info(ctx)
	if err != nil {
		return nil, err
	}

	return client, nil
}
