package strategy

import (
	"context"

	"github.com/cockroachdb/errors"

	"github.com/projecteru2/core/types"
)

// GlobalPlan spreads need workloads to keep Usage+Rate as even as possible across nodes.
func GlobalPlan(_ context.Context, infos []Info, need, total, _ int) (map[string]int, error) {
	if total < need {
		return nil, errors.Wrapf(types.ErrInsufficientResource, "need: %d, available: %d", need, total)
	}
	h := newInfoHeap(
		infos,
		func(a, b Info) bool { return (a.Usage + a.Rate) < (b.Usage + b.Rate) },
		func(info Info) bool { return info.Capacity > 0 },
	)

	deployMap, placed := h.place(need, func(info *Info) {
		info.Usage += info.Rate
		info.Capacity--
	})
	if placed < need {
		return nil, errors.Wrapf(types.ErrInsufficientResource, "need: %d, available: %d", need, placed)
	}
	return deployMap, nil
}
