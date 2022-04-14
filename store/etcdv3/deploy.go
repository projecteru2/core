package etcdv3

import (
	"context"
	"path/filepath"
	"strings"

	clientv3 "go.etcd.io/etcd/client/v3"

	"github.com/projecteru2/core/log"
)

// GetDeployStatus get deploy status from store
func (m *Mercury) GetDeployStatus(ctx context.Context, appname, entryname string) (map[string]int, error) {
	// 手动加 / 防止不精确
	key := filepath.Join(workloadDeployPrefix, appname, entryname) + "/"
	resp, err := m.Get(ctx, key, clientv3.WithPrefix(), clientv3.WithKeysOnly())
	if err != nil {
		return nil, err
	}
	if resp.Count == 0 {
		log.Warnf(ctx, "[MakeDeployStatus] Deploy status not found %s.%s", appname, entryname)
	}

	deployCount := m.doGetDeployStatus(ctx, resp)

	processingCount, err := m.doLoadProcessing(ctx, appname, entryname)
	if err != nil {
		return nil, err
	}

	// node count: deploy count + processing count
	nodeCount := map[string]int{}
	for node, count := range deployCount {
		nodeCount[node] = count
	}
	for node, count := range processingCount {
		nodeCount[node] += count
	}

	return nodeCount, nil
}

// doGetDeployStatus returns how many workload have been deployed on each node
func (m *Mercury) doGetDeployStatus(_ context.Context, resp *clientv3.GetResponse) map[string]int {
	nodesCount := map[string]int{}
	for _, ev := range resp.Kvs {
		key := string(ev.Key)
		parts := strings.Split(key, "/")
		nodename := parts[len(parts)-2]
		if _, ok := nodesCount[nodename]; !ok {
			nodesCount[nodename] = 1
			continue
		}
		nodesCount[nodename]++
	}

	return nodesCount
}
