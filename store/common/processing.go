package common

import (
	"context"
	"path/filepath"
	"strconv"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
)

func (s *Store) CreateProcessing(ctx context.Context, processing *types.Processing, count int) error {
	return s.Create(ctx, map[string]string{ProcessingKey(processing): strconv.Itoa(count)})
}

func (s *Store) DeleteProcessing(ctx context.Context, processing *types.Processing) error {
	return s.Delete(ctx, []string{ProcessingKey(processing)})
}

func (s *Store) doLoadProcessing(ctx context.Context, appname, entryname string) (map[string]int, error) {
	nodesCount := map[string]int{}
	// trailing slash keeps the prefix from matching a longer entrypoint
	data, err := s.GetPrefix(ctx, filepath.Join(WorkloadProcessingPrefix, appname, entryname)+"/", 0)
	if err != nil {
		return nil, err
	}
	logger := log.WithFunc("store.common.doLoadProcessing")

	for key, value := range data {
		count, err := strconv.Atoi(value)
		if err != nil {
			logger.Error(ctx, err, "load processing status")
			continue
		}
		nodesCount[ParseNodename(key)] += count
	}

	logger.Debugf(ctx, "processing result: %+v", nodesCount)
	return nodesCount, nil
}
