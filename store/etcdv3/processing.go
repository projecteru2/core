package etcdv3

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/sanity-io/litter"
	clientv3 "go.etcd.io/etcd/client/v3"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/strategy"
	"github.com/projecteru2/core/types"
)

func (m *Mercury) getProcessingKey(processing *types.Processing) string {
	return filepath.Join(workloadProcessingPrefix, processing.Appname, processing.Entryname, processing.Nodename, processing.Ident)
}

// CreateProcessing save processing status in etcd
func (m *Mercury) CreateProcessing(ctx context.Context, processing *types.Processing, count int) error {
	_, err := m.Create(ctx, m.getProcessingKey(processing), fmt.Sprintf("%d", count))
	return err
}

// DeleteProcessing delete processing status in etcd
func (m *Mercury) DeleteProcessing(ctx context.Context, processing *types.Processing) error {
	_, err := m.Delete(ctx, m.getProcessingKey(processing))
	return err
}

func (m *Mercury) doLoadProcessing(ctx context.Context, appname, entryname string, strategyInfos []strategy.Info) error {
	// 显式的加 / 保证 prefix 一致性
	processingKey := filepath.Join(workloadProcessingPrefix, appname, entryname) + "/"
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
			log.Errorf(ctx, "[doLoadProcessing] Load processing status failed %v", err)
			continue
		}
		if _, ok := nodesCount[nodename]; !ok {
			nodesCount[nodename] = count
			continue
		}
		nodesCount[nodename] += count
	}

	log.Debug(ctx, "[doLoadProcessing] Processing result:")
	litter.Dump(nodesCount)
	setCount(nodesCount, strategyInfos)
	return nil
}
