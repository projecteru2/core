package redis

import (
	"context"
	"maps"
	"path/filepath"
	"strings"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/store/common"
)

func (r *Rediaron) GetDeployStatus(ctx context.Context, appname, entryname string) (map[string]int, error) {
	// trailing slash keeps the prefix from matching a longer entrypoint
	key := filepath.Join(common.WorkloadDeployPrefix, appname, entryname) + "/*"
	data, err := r.getByKeyPattern(ctx, key, 0)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		log.WithFunc("store.redis.GetDeployStatus").Warnf(ctx, "deploy status not found %s.%s", appname, entryname)
	}

	deployCount := r.doGetDeployStatus(data)

	processingCount, err := r.doLoadProcessing(ctx, appname, entryname)
	if err != nil {
		return nil, err
	}

	nodeCount := map[string]int{}
	maps.Copy(nodeCount, deployCount)
	for node, count := range processingCount {
		nodeCount[node] += count
	}

	return nodeCount, nil
}

// doGetDeployStatus counts the deployed workloads per node.
func (r *Rediaron) doGetDeployStatus(data map[string]string) map[string]int {
	nodesCount := map[string]int{}
	for key := range data {
		parts := strings.Split(key, "/")
		nodename := parts[len(parts)-2]
		nodesCount[nodename]++
	}

	return nodesCount
}
