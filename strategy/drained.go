package strategy

import (
	"context"
	"slices"
	"sort"

	"github.com/projecteru2/core/types"

	"github.com/cockroachdb/errors"
)

// DrainedPlan fills the lowest-capacity nodes first, draining each before moving to the next.
func DrainedPlan(_ context.Context, infos []Info, need, total, _ int) (map[string]int, error) {
	if total < need {
		return nil, errors.Wrapf(types.ErrInsufficientResource, "need: %d, available: %d", need, total)
	}

	deploy := map[string]int{}

	infosCopy := slices.Clone(infos)
	sort.Slice(infosCopy, func(i, j int) bool {
		if infosCopy[i].Capacity < infosCopy[j].Capacity {
			return true
		}
		return infosCopy[i].Usage > infosCopy[j].Usage
	})

	for _, info := range infosCopy {
		deploy[info.Nodename] = min(need, info.Capacity)
		need -= deploy[info.Nodename]
		if need == 0 {
			return deploy, nil
		}
	}
	return nil, errors.Wrapf(types.ErrInsufficientResource, "BUG: never reach here")
}
