package strategy

import (
	"context"

	"github.com/cockroachdb/errors"

	"github.com/projecteru2/core/types"
)

// CommunismPlan spreads need workloads so the global per-node count ends as even as possible.
func CommunismPlan(_ context.Context, infos []Info, need, total, limit int) (map[string]int, error) {
	if total < need {
		return nil, errors.Wrapf(types.ErrInsufficientResource, "need: %d, available: %d", need, total)
	}

	iHeap := newInfoHeap(
		infos,
		func(a, b Info) bool {
			return a.Count < b.Count || (a.Count == b.Count && a.Capacity > b.Capacity)
		},
		func(info Info) bool {
			return info.Capacity != 0 && (limit <= 0 || info.Count < limit)
		},
	)
	deploy, placed := iHeap.place(need, func(info *Info) {
		info.Count++
		info.Capacity--
	})
	if placed < need {
		return nil, errors.Wrapf(types.ErrInsufficientResource, "reached nodelimit, a node can host at most %d instances", limit)
	}
	return deploy, nil
}
