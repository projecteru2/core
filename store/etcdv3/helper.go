package etcdv3

import (
	"net/http"
	"os"
	"strings"

	enginetypes "github.com/docker/docker/api/types"
	engineapi "github.com/docker/docker/client"
	"github.com/docker/go-connections/tlsconfig"
	enginemock "github.com/projecteru2/core/3rdmocks"
	"github.com/projecteru2/core/types"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/mock"
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

func makeMockClient() (engineapi.APIClient, error) {
	engine := &enginemock.APIClient{}
	engine.On("Info", mock.AnythingOfType("*context.emptyCtx")).Return(
		enginetypes.Info{NCPU: 1, MemTotal: types.GByte + 100}, nil)
	return engine, nil
}

func makeRawClient(endpoint, apiversion string) (engineapi.APIClient, error) {
	return engineapi.NewClient(endpoint, apiversion, nil, nil)
}

// use endpoint, cert files path, and api version to create docker client
// we don't check whether this is connectable
func makeRawClientWithTLS(ca, cert, key *os.File, endpoint, apiversion string) (engineapi.APIClient, error) {
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

func parseStatusKey(key string) (string, string, string, string) {
	parts := strings.Split(key, "/")
	l := len(parts)
	return parts[l-4], parts[l-3], parts[l-2], parts[l-1]
}

func setCount(nodesCount map[string]int, nodesInfo []types.NodeInfo) []types.NodeInfo {
	for p, nodeInfo := range nodesInfo {
		if v, ok := nodesCount[nodeInfo.Name]; ok {
			nodesInfo[p].Count += v
		}
	}
	return nodesInfo
}
