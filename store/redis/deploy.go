package redis

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/strategy"
	"github.com/projecteru2/core/types"
)

// MakeDeployStatus get deploy status from store
func (r *Rediaron) MakeDeployStatus(ctx context.Context, opts *types.DeployOptions, strategyInfos []strategy.Info) error {
	// 手动加 / 防止不精确
	key := filepath.Join(workloadDeployPrefix, opts.Name, opts.Entrypoint.Name) + "/*"
	data, err := r.getByKeyPattern(ctx, key, 0)
	if err != nil {
		return err
	}
	if len(data) == 0 {
		log.Warnf(ctx, "[MakeDeployStatus] Deploy status not found %s.%s", opts.Name, opts.Entrypoint.Name)
	}
	if err = r.doGetDeployStatus(ctx, data, strategyInfos); err != nil {
		return err
	}
	return r.doLoadProcessing(ctx, opts, strategyInfos)
}

func (r *Rediaron) doGetDeployStatus(_ context.Context, data map[string]string, strategyInfos []strategy.Info) error {
	nodesCount := map[string]int{}
	for key := range data {
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
