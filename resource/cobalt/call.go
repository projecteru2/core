package cobalt

import (
	"context"
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/resource/plugins"
)

func call[T any](ctx context.Context, ps []plugins.Plugin, f func(plugins.Plugin) (T, error)) (map[plugins.Plugin]T, error) {
	var wg sync.WaitGroup
	var combinedErr error
	var results sync.Map
	for _, p := range ps {
		wg.Add(1)
		go func(p plugins.Plugin) {
			defer wg.Done()

			result, err := f(p)
			if err != nil {
				log.WithFunc("resource.cobalt.call").Errorf(ctx, err, "failed to call plugin %+v", p.Name())
				results.Store(p, err)
				return
			}
			results.Store(p, result)
		}(p)
	}
	wg.Wait()
	ans := make(map[plugins.Plugin]T)
	results.Range(func(key, value any) bool {
		switch vt := value.(type) {
		case error:
			combinedErr = errors.CombineErrors(combinedErr, vt)
		case T:
			ans[key.(plugins.Plugin)] = vt
		}
		return true
	})
	return ans, combinedErr
}
