package strategy

import (
	"cmp"
	"context"
	"slices"

	"github.com/cockroachdb/errors"

	"github.com/projecteru2/core/types"
)

// DrainedPlan fills the lowest-capacity nodes first, draining each before moving to the next.
func DrainedPlan(_ context.Context, infos []Info, need, total, _ int) (map[string]int, error) {
	if total < need {
		return nil, errors.Wrapf(types.ErrInsufficientResource, "need: %d, available: %d", need, total)
	}

	deploy := map[string]int{}

	infosCopy := slices.Clone(infos)
	slices.SortFunc(infosCopy, func(a, b Info) int {
		return cmp.Or(cmp.Compare(a.Capacity, b.Capacity), cmp.Compare(b.Usage, a.Usage))
	})

	for _, info := range infosCopy {
		deploy[info.Nodename] = min(need, info.Capacity)
		need -= deploy[info.Nodename]
		if need == 0 {
			return deploy, nil
		}
	}
	return nil, errors.Wrap(types.ErrInsufficientResource, "not enough node capacity to satisfy the plan")
}
