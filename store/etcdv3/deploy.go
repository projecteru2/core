package etcdv3

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/projecteru2/core/types"
	log "github.com/sirupsen/logrus"
	"go.etcd.io/etcd/v3/clientv3"
)

// MakeDeployStatus get deploy status from store
func (m *Mercury) MakeDeployStatus(ctx context.Context, opts *types.DeployOptions, strategyInfos []types.StrategyInfo) error {
	// 手动加 / 防止不精确
	key := filepath.Join(containerDeployPrefix, opts.Name, opts.Entrypoint.Name) + "/"
	resp, err := m.Get(ctx, key, clientv3.WithPrefix(), clientv3.WithKeysOnly())
	if err != nil {
		return err
	}
	if resp.Count == 0 {
		log.Warnf("[MakeDeployStatus] Deploy status not found %s.%s", opts.Name, opts.Entrypoint.Name)
	}
	if err = m.doGetDeployStatus(ctx, resp, strategyInfos); err != nil {
		return err
	}
	return m.doLoadProcessing(ctx, opts, strategyInfos)
}

func (m *Mercury) doGetDeployStatus(_ context.Context, resp *clientv3.GetResponse, strategyInfos []types.StrategyInfo) error {
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
