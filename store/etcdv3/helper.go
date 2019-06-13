package etcdv3

import (
	"os"
	"strings"

	"github.com/projecteru2/core/engine"
	"github.com/projecteru2/core/engine/docker"
	fakeengine "github.com/projecteru2/core/engine/mocks/fakeengine"
	"github.com/projecteru2/core/engine/virt"
	"github.com/projecteru2/core/types"
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

func makeMockClient() (engine.API, error) {
	return fakeengine.MakeMockClient(), nil
}

func makeDockerClient(config types.Config, endpoint, apiversion string) (engine.API, error) {
	return docker.MakeRawClient(config, nil, endpoint, apiversion)
}

// use endpoint, cert files path, and api version to create docker client
// we don't check whether this is connectable
func makeDockerClientWithTLS(config types.Config, ca, cert, key *os.File, endpoint, apiversion string) (engine.API, error) {
	return docker.MakeRawClientWithTLS(config, ca, cert, key, endpoint, apiversion)
}

func makeVirtClient(config types.Config, endpoint, apiversion string) (engine.API, error) {
	host := endpoint[len(nodeVirtPrefixKey):]
	return virt.MakeClient(config, host, apiversion)
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
