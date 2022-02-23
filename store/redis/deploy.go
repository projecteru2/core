package redis

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/projecteru2/core/log"
)

// GetDeployStatus .
func (r *Rediaron) GetDeployStatus(ctx context.Context, appname, entryname string) (map[string]int, error) {
	// 手动加 / 防止不精确
	key := filepath.Join(workloadDeployPrefix, appname, entryname) + "/*"
	data, err := r.getByKeyPattern(ctx, key, 0)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		log.Warnf(ctx, "[MakeDeployStatus] Deploy status not found %s.%s", appname, entryname)
	}

	deployCount := r.doGetDeployStatus(ctx, data)

	processingCount, err := r.doLoadProcessing(ctx, appname, entryname)
	if err != nil {
		return nil, err
	}

	// node count: deploy count + processing count
	nodeCount := map[string]int{}
	for node, count := range deployCount {
		nodeCount[node] = count
	}
	for node, count := range processingCount {
		nodeCount[node] += count
	}

	return nodeCount, nil
}

// doGetDeployStatus returns how many workload have been deployed on each node
func (r *Rediaron) doGetDeployStatus(_ context.Context, data map[string]string) map[string]int {
	nodesCount := map[string]int{}
	for key := range data {
		parts := strings.Split(key, "/")
		nodename := parts[len(parts)-2]
		nodesCount[nodename]++
	}

	return nodesCount
}
