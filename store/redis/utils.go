package redis

import (
	"context"
)

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
