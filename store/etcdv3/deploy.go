package etcdv3

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/coreos/etcd/clientv3"
	"github.com/projecteru2/core/types"
	log "github.com/sirupsen/logrus"
)

// MakeDeployStatus get deploy status from store
func (m *Mercury) MakeDeployStatus(ctx context.Context, opts *types.DeployOptions, nodesInfo []types.NodeInfo) ([]types.NodeInfo, error) {
	key := filepath.Join(containerDeployPrefix, opts.Name, opts.Entrypoint.Name)
	resp, err := m.Get(ctx, key, clientv3.WithPrefix(), clientv3.WithKeysOnly())
	if err != nil {
		return nil, err
	}
	if resp.Count != 0 {
		nodesInfo, err = m.doGetDeployStatus(ctx, resp, nodesInfo)
		if err != nil {
			return nil, err
		}
	} else {
		log.Warnf("[MakeDeployStatus] Deploy status not found %s", opts.Name)
	}
	return m.loadProcessing(ctx, opts, nodesInfo)
}

func (m *Mercury) doGetDeployStatus(ctx context.Context, resp *clientv3.GetResponse, nodesInfo []types.NodeInfo) ([]types.NodeInfo, error) {
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

	return setCount(nodesCount, nodesInfo), nil
}

// Obsolete, for old eru, container info not store by container meta, so we have to generate it
func (m *Mercury) doMakeDeployStatus(ctx context.Context, opts *types.DeployOptions, nodesInfo []types.NodeInfo) ([]types.NodeInfo, error) {
	resp, err := m.Get(ctx, fmt.Sprintf(containerInfoKey, ""), clientv3.WithPrefix())
	if err != nil {
		return nodesInfo, err
	}

	prefix := fmt.Sprintf("%s_%s", opts.Name, opts.Entrypoint.Name)
	container := &types.Container{}
	nodesCount := map[string]int{}
	for _, ev := range resp.Kvs {
		if err := json.Unmarshal(ev.Value, &container); err != nil {
			return nodesInfo, err
		}
		if container.Podname != opts.Podname {
			continue
		}
		if !strings.HasPrefix(container.Name, prefix) {
			continue
		}
		if _, ok := nodesCount[container.Nodename]; !ok {
			nodesCount[container.Nodename] = 1
			continue
		}
		nodesCount[container.Nodename]++
	}
	return setCount(nodesCount, nodesInfo), nil
}
