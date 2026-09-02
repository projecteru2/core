package common

import (
	"context"
	"path/filepath"

	"golang.org/x/sync/errgroup"

	"github.com/projecteru2/core/log"
)

func (s *Store) GetDeployStatus(ctx context.Context, appname, entryname string) (map[string]int, error) {
	var keys []string
	var processingCount map[string]int
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() (err error) {
		// trailing slash keeps the prefix from matching a longer entrypoint
		keys, err = s.ListPrefix(gctx, filepath.Join(WorkloadDeployPrefix, appname, entryname)+"/")
		return err
	})
	g.Go(func() (err error) {
		processingCount, err = s.doLoadProcessing(gctx, appname, entryname)
		return err
	})
	if err := g.Wait(); err != nil {
		return nil, err
	}
	if len(keys) == 0 {
		log.WithFunc("store.common.GetDeployStatus").Warnf(ctx, "deploy status not found %s.%s", appname, entryname)
	}

	nodeCount := map[string]int{}
	for _, key := range keys {
		nodeCount[ParseNodename(key)]++
	}
	for node, count := range processingCount {
		nodeCount[node] += count
	}

	return nodeCount, nil
}
