package etcdv3

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/coreos/etcd/clientv3"
	"github.com/projecteru2/core/types"
	log "github.com/sirupsen/logrus"
)

// SaveProcessing save processing status in etcd
func (m *Mercury) SaveProcessing(ctx context.Context, opts *types.DeployOptions, nodeInfo types.NodeInfo) error {
	processingKey := filepath.Join(containerProcessingPrefix, opts.Name, opts.Entrypoint.Name, nodeInfo.Name, opts.ProcessIdent)
	_, err := m.Create(ctx, processingKey, fmt.Sprintf("%d", nodeInfo.Deploy))
	return err
}

// UpdateProcessing update processing status in etcd
func (m *Mercury) UpdateProcessing(ctx context.Context, opts *types.DeployOptions, nodename string, count int) error {
	processingKey := filepath.Join(containerProcessingPrefix, opts.Name, opts.Entrypoint.Name, nodename, opts.ProcessIdent)
	_, err := m.Update(ctx, processingKey, fmt.Sprintf("%d", count))
	return err
}

// DeleteProcessing delete processing status in etcd
func (m *Mercury) DeleteProcessing(ctx context.Context, opts *types.DeployOptions, nodeInfo types.NodeInfo) error {
	processingKey := filepath.Join(containerProcessingPrefix, opts.Name, opts.Entrypoint.Name, nodeInfo.Name, opts.ProcessIdent)
	_, err := m.Delete(ctx, processingKey)
	return err
}

func (m *Mercury) loadProcessing(ctx context.Context, opts *types.DeployOptions, nodesInfo []types.NodeInfo) ([]types.NodeInfo, error) {
	processingKey := filepath.Join(containerProcessingPrefix, opts.Name, opts.Entrypoint.Name)
	resp, err := m.Get(ctx, processingKey, clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}

	if resp.Count == 0 {
		return nodesInfo, nil
	}
	nodesCount := map[string]int{}
	for _, ev := range resp.Kvs {
		key := string(ev.Key)
		parts := strings.Split(key, "/")
		nodename := parts[len(parts)-2]
		count, err := strconv.Atoi(string(ev.Value))
		if err != nil {
			log.Errorf("[loadProcessing] load processing status failed %v", err)
			continue
		}
		if _, ok := nodesCount[nodename]; !ok {
			nodesCount[nodename] = count
			continue
		}
		nodesCount[nodename] += count
	}

	return setCount(nodesCount, nodesInfo), nil
}
