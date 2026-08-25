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
	results := make([]T, len(ps))
	errs := make([]error, len(ps))
	for i, p := range ps {
		wg.Go(func() {
			if results[i], errs[i] = f(p); errs[i] != nil {
				log.WithFunc("resource.cobalt.call").Errorf(ctx, errs[i], "failed to call plugin %+v", p.Name())
			}
		})
	}
	wg.Wait()

	var combinedErr error
	ans := make(map[plugins.Plugin]T, len(ps))
	for i, p := range ps {
		if errs[i] != nil {
			combinedErr = errors.CombineErrors(combinedErr, errs[i])
			continue
		}
		ans[p] = results[i]
	}
	return ans, combinedErr
}
