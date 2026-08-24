package strategy

import (
	"context"
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

	infosCopy := make([]Info, len(infos))
	copy(infosCopy, infos)
	sort.Slice(infosCopy, func(i, j int) bool {
		if infosCopy[i].Capacity < infosCopy[j].Capacity {
			return true
		}
		return infosCopy[i].Usage > infosCopy[j].Usage
	})

	for idx := range infosCopy {
		info := &infosCopy[idx]
		if need < info.Capacity {
			deploy[info.Nodename] = need
			need = 0
		} else {
			deploy[info.Nodename] = info.Capacity
			need -= info.Capacity
		}
		if need == 0 {
			return deploy, nil
		}
	}
	return nil, errors.Wrapf(types.ErrInsufficientResource, "BUG: never reach here")
}
