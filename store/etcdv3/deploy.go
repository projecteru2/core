package etcdv3

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/strategy"
	"github.com/projecteru2/core/types"
	"go.etcd.io/etcd/v3/clientv3"
)

// MakeDeployStatus get deploy status from store
func (m *Mercury) MakeDeployStatus(ctx context.Context, processing *types.Processing, strategyInfos []strategy.Info) error {
	// 手动加 / 防止不精确
	key := filepath.Join(workloadDeployPrefix, processing.Appname, processing.Entryname) + "/"
	resp, err := m.Get(ctx, key, clientv3.WithPrefix(), clientv3.WithKeysOnly())
	if err != nil {
		return err
	}
	if resp.Count == 0 {
		log.Warnf(ctx, "[MakeDeployStatus] Deploy status not found %s.%s", processing.Appname, processing.Entryname)
	}
	if err = m.doGetDeployStatus(ctx, resp, strategyInfos); err != nil {
		return err
	}
	return m.doLoadProcessing(ctx, processing, strategyInfos)
}

func (m *Mercury) doGetDeployStatus(_ context.Context, resp *clientv3.GetResponse, strategyInfos []strategy.Info) error {
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

	setCount(nodesCount, strategyInfos)
	return nil
}
