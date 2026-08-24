package etcdv3

import (
	"context"
	"maps"
	"path/filepath"
	"strings"

	clientv3 "go.etcd.io/etcd/client/v3"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/store/common"
)

func (m *Mercury) GetDeployStatus(ctx context.Context, appname, entryname string) (map[string]int, error) {
	// trailing slash keeps the prefix from matching a longer entrypoint
	key := filepath.Join(common.WorkloadDeployPrefix, appname, entryname) + "/"
	resp, err := m.Get(ctx, key, clientv3.WithPrefix(), clientv3.WithKeysOnly())
	if err != nil {
		return nil, err
	}
	if resp.Count == 0 {
		log.WithFunc("store.etcdv3.GetDeployStatus").Warnf(ctx, "deploy status not found %s.%s", appname, entryname)
	}

	deployCount := m.doGetDeployStatus(resp)
	processingCount, err := m.doLoadProcessing(ctx, appname, entryname)
	if err != nil {
		return nil, err
	}

	nodeCount := map[string]int{}
	maps.Copy(nodeCount, deployCount)
	for node, count := range processingCount {
		nodeCount[node] += count
	}

	return nodeCount, nil
}

// doGetDeployStatus counts the deployed workloads per node.
func (m *Mercury) doGetDeployStatus(resp *clientv3.GetResponse) map[string]int {
	nodesCount := map[string]int{}
	for _, ev := range resp.Kvs {
		key := string(ev.Key)
		parts := strings.Split(key, "/")
		nodesCount[parts[len(parts)-2]]++
	}

	return nodesCount
}
