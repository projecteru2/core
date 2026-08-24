package etcdv3

import (
	"context"
	"path/filepath"
	"strconv"
	"strings"

	clientv3 "go.etcd.io/etcd/client/v3"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/store/common"
	"github.com/projecteru2/core/types"
)

func (m *Mercury) CreateProcessing(ctx context.Context, processing *types.Processing, count int) error {
	_, err := m.Create(ctx, common.ProcessingKey(processing), strconv.Itoa(count))
	return err
}

func (m *Mercury) DeleteProcessing(ctx context.Context, processing *types.Processing) error {
	_, err := m.Delete(ctx, common.ProcessingKey(processing))
	return err
}

func (m *Mercury) doLoadProcessing(ctx context.Context, appname, entryname string) (map[string]int, error) {
	nodesCount := map[string]int{}
	// trailing slash keeps the prefix from matching a longer entrypoint
	processingKey := filepath.Join(common.WorkloadProcessingPrefix, appname, entryname) + "/"
	resp, err := m.Get(ctx, processingKey, clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}
	if resp.Count == 0 {
		return nodesCount, nil
	}
	logger := log.WithFunc("store.etcdv3.doLoadProcessing")

	for _, ev := range resp.Kvs {
		key := string(ev.Key)
		parts := strings.Split(key, "/")
		nodename := parts[len(parts)-2]
		count, err := strconv.Atoi(string(ev.Value))
		if err != nil {
			logger.Error(ctx, err, "load processing status")
			continue
		}
		nodesCount[nodename] += count
	}
	logger.Debugf(ctx, "processing result: %+v", nodesCount)
	return nodesCount, nil
}
