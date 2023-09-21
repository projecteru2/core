package cobalt

import (
	"context"
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/resource/plugins"
)

func call[T any](ctx context.Context, ps []plugins.Plugin, f func(plugins.Plugin) (T, error)) (map[plugins.Plugin]T, error) {
	var mu sync.Mutex
	var wg sync.WaitGroup
	var combinedErr error
	results := map[plugins.Plugin]T{}

	for _, p := range ps {
		wg.Add(1)
		go func(p plugins.Plugin) {
			defer wg.Done()

			result, err := f(p)
			mu.Lock()
			defer mu.Unlock()

			if err != nil {
				log.WithFunc("resource.cobalt.call").Errorf(ctx, err, "failed to call plugin %+v", p.Name())
				combinedErr = errors.CombineErrors(combinedErr, errors.Wrap(err, p.Name()))
				return
			}
			results[p] = result
		}(p)
	}
	wg.Wait()
	return results, combinedErr
}
