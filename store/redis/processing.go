package redis

import (
	"context"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/sanity-io/litter"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/strategy"
	"github.com/projecteru2/core/types"
)

// SaveProcessing save processing status in etcd
func (r *Rediaron) SaveProcessing(ctx context.Context, opts *types.DeployOptions, nodename string, count int) error {
	processingKey := filepath.Join(workloadProcessingPrefix, opts.Name, opts.Entrypoint.Name, nodename, opts.ProcessIdent)
	return r.BatchCreate(ctx, map[string]string{processingKey: strconv.Itoa(count)})
}

// UpdateProcessing update processing status in etcd
func (r *Rediaron) UpdateProcessing(ctx context.Context, opts *types.DeployOptions, nodename string, count int) error {
	processingKey := filepath.Join(workloadProcessingPrefix, opts.Name, opts.Entrypoint.Name, nodename, opts.ProcessIdent)
	return r.BatchUpdate(ctx, map[string]string{processingKey: strconv.Itoa(count)})
}

// DeleteProcessing delete processing status in etcd
func (r *Rediaron) DeleteProcessing(ctx context.Context, opts *types.DeployOptions, nodename string) error {
	processingKey := filepath.Join(workloadProcessingPrefix, opts.Name, opts.Entrypoint.Name, nodename, opts.ProcessIdent)
	return r.BatchDelete(ctx, []string{processingKey})
}

func (r *Rediaron) doLoadProcessing(ctx context.Context, opts *types.DeployOptions, strategyInfos []strategy.Info) error {
	// 显式的加 / 保证 prefix 一致性
	processingKey := filepath.Join(workloadProcessingPrefix, opts.Name, opts.Entrypoint.Name) + "/*"
	data, err := r.getByKeyPattern(ctx, processingKey, 0)
	if err != nil {
		return err
	}
	if len(data) == 0 {
		return nil
	}

	nodesCount := map[string]int{}
	for k, v := range data {
		parts := strings.Split(k, "/")
		nodename := parts[len(parts)-2]
		count, err := strconv.Atoi(v)
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
