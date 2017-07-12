package etcdstore

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	etcdclient "github.com/coreos/etcd/client"
	"gitlab.ricebook.net/platform/core/types"
)

func (k *krypton) MakeDeployStatus(opts *types.DeployOptions, nodesInfo []types.NodeInfo) ([]types.NodeInfo, error) {
	resp, err := k.etcd.Get(context.Background(), allContainerKey, &etcdclient.GetOptions{Recursive: true})
	if err != nil {
		if etcdclient.IsKeyNotFound(err) {
			return nodesInfo, nil
		}
		return nodesInfo, err
	}

	prefix := fmt.Sprintf("%s_%s", opts.Appname, opts.Entrypoint)
	m := map[string]string{}
	nodesCount := map[string]int64{}
	for _, node := range resp.Node.Nodes {
		json.Unmarshal([]byte(node.Value), &m)
		if m["podname"] != opts.Podname {
			continue
		}
		if !strings.HasPrefix(m["name"], prefix) {
			continue
		}
		if _, ok := nodesCount[m["nodename"]]; !ok {
			nodesCount[m["nodename"]] = 1
			continue
		}
		nodesCount[m["nodename"]] += 1
	}

	for p, nodeInfo := range nodesInfo {
		if v, ok := nodesCount[nodeInfo.Name]; ok {
			nodesInfo[p].Count = v
		}
	}

	return nodesInfo, nil
}
