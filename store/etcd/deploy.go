package etcdstore

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	etcdclient "github.com/coreos/etcd/client"
	"github.com/projecteru2/core/types"
)

// MakeDeployStatus get deploy status from store
func (k *Krypton) MakeDeployStatus(ctx context.Context, opts *types.DeployOptions, nodesInfo []types.NodeInfo) ([]types.NodeInfo, error) {
	key := filepath.Join(containerDeployPrefix, opts.Name, opts.Entrypoint.Name)
	resp, err := k.etcd.Get(ctx, key, &etcdclient.GetOptions{Recursive: true})
	if err == nil {
		nodesInfo, err = k.doGetDeployStatus(ctx, resp, nodesInfo)
	} else if etcdclient.IsKeyNotFound(err) {
		nodesInfo, err = k.doMakeDeployStatus(ctx, opts, nodesInfo)
	} else {
		return nodesInfo, err
	}
	return k.loadProcessing(ctx, opts, nodesInfo)
}

func (k *Krypton) doGetDeployStatus(ctx context.Context, resp *etcdclient.Response, nodesInfo []types.NodeInfo) ([]types.NodeInfo, error) {
	nodesCount := map[string]int{}
	for _, node := range resp.Node.Nodes {
		nodeName := strings.Trim(strings.TrimPrefix(node.Key, resp.Node.Key), "/")
		nodesCount[nodeName] = len(node.Nodes)
	}

	for p, nodeInfo := range nodesInfo {
		if v, ok := nodesCount[nodeInfo.Name]; ok {
			nodesInfo[p].Count = v
		}
	}
	return nodesInfo, nil
}

func (k *Krypton) doMakeDeployStatus(ctx context.Context, opts *types.DeployOptions, nodesInfo []types.NodeInfo) ([]types.NodeInfo, error) {
	resp, err := k.etcd.Get(ctx, allContainersKey, &etcdclient.GetOptions{Recursive: true})
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
