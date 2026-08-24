package redis

import (
	"context"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
)

func (r *Rediaron) CreateProcessing(ctx context.Context, processing *types.Processing, count int) error {
	processingKey := r.getProcessingKey(processing)
	return r.BatchCreate(ctx, map[string]string{processingKey: strconv.Itoa(count)})
}

func (r *Rediaron) DeleteProcessing(ctx context.Context, processing *types.Processing) error {
	return r.BatchDelete(ctx, []string{r.getProcessingKey(processing)})
}

func (r *Rediaron) getProcessingKey(processing *types.Processing) string {
	return filepath.Join(workloadProcessingPrefix, processing.Appname, processing.Entryname, processing.Nodename, processing.Ident)
}

// doLoadProcessing counts the in-flight workloads per node.
func (r *Rediaron) doLoadProcessing(ctx context.Context, appname, entryname string) (map[string]int, error) {
	nodesCount := map[string]int{}
	// trailing slash keeps the prefix from matching a longer entrypoint
	processingKey := filepath.Join(workloadProcessingPrefix, appname, entryname) + "/*"
	data, err := r.getByKeyPattern(ctx, processingKey, 0)
	if err != nil {
		return nil, err
	}

	if len(data) == 0 {
		return nodesCount, nil
	}
	logger := log.WithFunc("store.redis.doLoadProcessing")

	for k, v := range data {
		parts := strings.Split(k, "/")
		nodename := parts[len(parts)-2]
		count, err := strconv.Atoi(v)
		if err != nil {
			logger.Error(ctx, err, "load processing status")
			continue
		}
		nodesCount[nodename] += count
	}

	logger.Debugf(ctx, "processing result: %+v", nodesCount)
	return nodesCount, nil
}
