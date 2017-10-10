package etcdstore

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	etcdclient "github.com/coreos/etcd/client"
	"github.com/projecteru2/core/types"
)

func (k *krypton) MakeDeployStatus(opts *types.DeployOptions, nodesInfo []types.NodeInfo) ([]types.NodeInfo, error) {
	key := fmt.Sprintf(containerDeployStatusKey, opts.Name, opts.Entrypoint.Name)
	resp, err := k.etcd.Get(context.Background(), key, &etcdclient.GetOptions{Recursive: true})
	if err != nil && etcdclient.IsKeyNotFound(err) {
		return k.doMakeDeployStatus(opts, nodesInfo)
	}
	return k.doGetDeployStatus(resp, nodesInfo)
}

func (k *krypton) doGetDeployStatus(resp *etcdclient.Response, nodesInfo []types.NodeInfo) ([]types.NodeInfo, error) {
	nodesCount := map[string]int{}
	for _, node := range resp.Node.Nodes {
		nodeName := strings.Trim(node.Key, resp.Node.Key)
		nodesCount[nodeName] = len(node.Nodes)
	}

	for p, nodeInfo := range nodesInfo {
		if v, ok := nodesCount[nodeInfo.Name]; ok {
			nodesInfo[p].Count = v
		}
	}
	return nodesInfo, nil
}

func (k *krypton) doMakeDeployStatus(opts *types.DeployOptions, nodesInfo []types.NodeInfo) ([]types.NodeInfo, error) {
	resp, err := k.etcd.Get(context.Background(), allContainerKey, &etcdclient.GetOptions{Recursive: true})
	if err != nil {
		if etcdclient.IsKeyNotFound(err) {
			return nodesInfo, nil
		}
		return nodesInfo, err
	}

	prefix := fmt.Sprintf("%s_%s", opts.Name, opts.Entrypoint.Name)
	m := map[string]string{}
	nodesCount := map[string]int{}
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
		nodesCount[m["nodename"]]++
	}

	for p, nodeInfo := range nodesInfo {
		if v, ok := nodesCount[nodeInfo.Name]; ok {
			nodesInfo[p].Count = v
		}
	}

	return nodesInfo, nil
}
