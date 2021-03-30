package redis

import (
	"context"
	"strings"

	"github.com/projecteru2/core/strategy"
)

// extracts node name from key
// /nodestatus/nodename -> nodename
func extractNodename(s string) string {
	ps := strings.Split(s, "/")
	return ps[len(ps)-1]
}

func parseStatusKey(key string) (string, string, string, string) {
	parts := strings.Split(key, "/")
	l := len(parts)
	return parts[l-4], parts[l-3], parts[l-2], parts[l-1]
}

func setCount(nodesCount map[string]int, strategyInfos []strategy.Info) {
	for i, strategyInfo := range strategyInfos {
		if v, ok := nodesCount[strategyInfo.Nodename]; ok {
			strategyInfos[i].Count += v
		}
	}
}

// getByKeyPattern gets key-value pairs that key matches pattern
func (r *Rediaron) getByKeyPattern(ctx context.Context, pattern string, limit int64) (map[string]string, error) {
	var (
		cursor uint64
		result []string
		err    error
		count  int64
		keys   = []string{}
	)
	for {
		result, cursor, err = r.cli.Scan(ctx, cursor, pattern, 0).Result()
		if err != nil {
			return nil, err
		}

		keys = append(keys, result...)
		count += int64(len(result))
		if cursor == 0 || (limit > 0 && count >= limit) {
			break
		}
	}
	if limit > 0 && int64(len(keys)) >= limit {
		keys = keys[:limit]
	}
	return r.GetMulti(ctx, keys)
}
