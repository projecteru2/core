package etcdv3

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/sanity-io/litter"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/strategy"
	"github.com/projecteru2/core/types"
	"go.etcd.io/etcd/clientv3"
)

// SaveProcessing save processing status in etcd
func (m *Mercury) SaveProcessing(ctx context.Context, opts *types.DeployOptions, nodename string, count int) error {
	processingKey := filepath.Join(workloadProcessingPrefix, opts.Name, opts.Entrypoint.Name, nodename, opts.ProcessIdent)
	_, err := m.Create(ctx, processingKey, fmt.Sprintf("%d", count))
	return err
}

// UpdateProcessing update processing status in etcd
func (m *Mercury) UpdateProcessing(ctx context.Context, opts *types.DeployOptions, nodename string, count int) error {
	processingKey := filepath.Join(workloadProcessingPrefix, opts.Name, opts.Entrypoint.Name, nodename, opts.ProcessIdent)
	_, err := m.Update(ctx, processingKey, fmt.Sprintf("%d", count))
	return err
}

// DeleteProcessing delete processing status in etcd
func (m *Mercury) DeleteProcessing(ctx context.Context, opts *types.DeployOptions, nodename string) error {
	processingKey := filepath.Join(workloadProcessingPrefix, opts.Name, opts.Entrypoint.Name, nodename, opts.ProcessIdent)
	_, err := m.Delete(ctx, processingKey)
	return err
}

func (m *Mercury) doLoadProcessing(ctx context.Context, opts *types.DeployOptions, strategyInfos []strategy.Info) error {
	// 显式的加 / 保证 prefix 一致性
	processingKey := filepath.Join(workloadProcessingPrefix, opts.Name, opts.Entrypoint.Name) + "/"
	resp, err := m.Get(ctx, processingKey, clientv3.WithPrefix())
	if err != nil {
		return err
	}

	if resp.Count == 0 {
		return nil
	}
	nodesCount := map[string]int{}
	for _, ev := range resp.Kvs {
		key := string(ev.Key)
		parts := strings.Split(key, "/")
		nodename := parts[len(parts)-2]
		count, err := strconv.Atoi(string(ev.Value))
		if err != nil {
			log.Errorf("[doLoadProcessing] Load processing status failed %v", err)
			continue
		}
		if _, ok := nodesCount[nodename]; !ok {
			nodesCount[nodename] = count
			continue
		}
		nodesCount[nodename] += count
	}

	log.Debug("[doLoadProcessing] Processing result:")
	litter.Dump(nodesCount)
	setCount(nodesCount, strategyInfos)
	return nil
}
