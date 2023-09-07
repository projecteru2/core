package strategy

import (
	"context"
	"sort"

	"github.com/projecteru2/core/types"

	"github.com/cockroachdb/errors"
)

// DrainedPlan 优先往Capacity最小的节点部署，尽可能把节点的资源榨干在部署下一台.
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

	for idx := 0; idx < len(infosCopy); idx++ {
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
