package common

import (
	"context"
	"path/filepath"

	"github.com/projecteru2/core/log"
)

func (s *Store) GetDeployStatus(ctx context.Context, appname, entryname string) (map[string]int, error) {
	// trailing slash keeps the prefix from matching a longer entrypoint
	keys, err := s.ListPrefix(ctx, filepath.Join(WorkloadDeployPrefix, appname, entryname)+"/")
	if err != nil {
		return nil, err
	}
	if len(keys) == 0 {
		log.WithFunc("store.common.GetDeployStatus").Warnf(ctx, "deploy status not found %s.%s", appname, entryname)
	}

	nodeCount := map[string]int{}
	for _, key := range keys {
		nodeCount[ParseNodename(key)]++
	}

	processingCount, err := s.doLoadProcessing(ctx, appname, entryname)
	if err != nil {
		return nil, err
	}
	for node, count := range processingCount {
		nodeCount[node] += count
	}

	return nodeCount, nil
}
