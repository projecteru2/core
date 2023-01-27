package cobalt

import (
	"context"

	"github.com/cockroachdb/errors"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/resource/plugins"
)

func call[T any](ctx context.Context, ps []plugins.Plugin, f func(plugins.Plugin) (T, error)) (map[plugins.Plugin]T, error) {
	// TODO 并行化，意义不大
	var combinedErr error
	results := map[plugins.Plugin]T{}

	for _, p := range ps {
		result, err := f(p)
		if err != nil {
			log.WithFunc("resource.cobalt.call").Errorf(ctx, err, "failed to call plugin %+v", p.Name())
			combinedErr = errors.CombineErrors(combinedErr, errors.Wrap(err, p.Name()))
			continue
		}
		results[p] = result
	}

	return results, combinedErr
}
