package redis

import (
	"context"
)

func (r *Rediaron) scanKeys(ctx context.Context, pattern string, limit int64) ([]string, error) {
	var cursor uint64
	seen := map[string]struct{}{}
	keys := []string{}
	for {
		result, next, err := r.cli.Scan(ctx, cursor, pattern, scanCount).Result()
		if err != nil {
			return nil, err
		}
		cursor = next
		for _, key := range result {
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			keys = append(keys, key)
		}
		if cursor == 0 || (limit > 0 && int64(len(keys)) >= limit) {
			break
		}
	}
	if limit > 0 && int64(len(keys)) > limit {
		keys = keys[:limit]
	}
	return keys, nil
}
