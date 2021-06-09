package redis

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/strategy"
)

// MakeDeployStatus get deploy status from store
func (r *Rediaron) MakeDeployStatus(ctx context.Context, appname, entryname string, strategyInfos []strategy.Info) error {
	// 手动加 / 防止不精确
	key := filepath.Join(workloadDeployPrefix, appname, entryname) + "/*"
	data, err := r.getByKeyPattern(ctx, key, 0)
	if err != nil {
		return err
	}
	if len(data) == 0 {
		log.Warnf(ctx, "[MakeDeployStatus] Deploy status not found %s.%s", appname, entryname)
	}
	if err = r.doGetDeployStatus(ctx, data, strategyInfos); err != nil {
		return err
	}
	return r.doLoadProcessing(ctx, appname, entryname, strategyInfos)
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
