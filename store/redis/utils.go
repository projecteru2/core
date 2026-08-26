package redis

import (
	"context"
)

func (r *Rediaron) scanKeys(ctx context.Context, pattern string, limit int64) ([]string, error) {
	var cursor uint64
	keys := []string{}
	for {
		result, next, err := r.cli.Scan(ctx, cursor, pattern, 0).Result()
		if err != nil {
			return nil, err
		}
		cursor = next
		keys = append(keys, result...)
		if cursor == 0 || (limit > 0 && int64(len(keys)) >= limit) {
			break
		}
	}
	if limit > 0 && int64(len(keys)) >= limit {
		keys = keys[:limit]
	}
	return keys, nil
}
