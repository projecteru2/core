package etcdstore

import (
	"net/http"
	"os"

	etcdclient "github.com/coreos/etcd/client"
	engineapi "github.com/docker/docker/client"
	"github.com/docker/go-connections/tlsconfig"
	log "github.com/sirupsen/logrus"
)

func dumpFromString(ca, cert, key *os.File, caStr, certStr, keyStr string) error {
	files := []*os.File{ca, cert, key}
	data := []string{caStr, certStr, keyStr}
	for i := 0; i < 3; i++ {
		if _, err := files[i].WriteString(data[i]); err != nil {
			return err
		}
		if err := files[i].Chmod(0444); err != nil {
			return err
		}
		if err := files[i].Close(); err != nil {
			return err
		}
	}
	log.Debug("[dumpFromString] Dump ca.pem, cert.pem, key.pem from string")
	return nil
}

func makeRawClient(endpoint, apiversion string) (*engineapi.Client, error) {
	return engineapi.NewClient(endpoint, apiversion, nil, nil)
}

// use endpoint, cert files path, and api version to create docker client
// we don't check whether this is connectable
func makeRawClientWithTLS(ca, cert, key *os.File, endpoint, apiversion string) (*engineapi.Client, error) {
	var cli *http.Client
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
	cli = &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tlsc,
		},
	}
	log.Debugf("[makeRawClientWithTLS] Create new http.Client for %s, %s", endpoint, apiversion)
	return engineapi.NewClient(endpoint, apiversion, cli, nil)
}

func getContainerDeployData(prefix string, nodes etcdclient.Nodes) []string {
	result := []string{}
	for _, node := range nodes {
		if len(node.Nodes) > 0 {
			result = append(result, getContainerDeployData(node.Key, node.Nodes)...)
		} else if !node.Dir {
			key := node.Key[len(prefix)+1:]
			result = append(result, key)
		}
	}
	return result
}
