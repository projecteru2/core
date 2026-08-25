package strategy

import (
	"context"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
)

const (
	Auto    = "AUTO"
	Fill    = "FILL"
	Each    = "EACH"
	Global  = "GLOBAL"
	Drained = "DRAINED"
	// Dummy marks a capacity-only request and has no entry in Plans
	Dummy = "DUMMY"
)

var Plans = map[string]strategyFunc{
	Auto:    CommunismPlan,
	Fill:    FillPlan,
	Each:    AveragePlan,
	Global:  GlobalPlan,
	Drained: DrainedPlan,
}

type Info struct {
	Nodename string

	Usage float64
	Rate  float64

	Capacity int
	Count    int
}

type strategyFunc = func(_ context.Context, _ []Info, need, total, limit int) (map[string]int, error)

func Deploy(ctx context.Context, strategy string, count, nodesLimit int, strategyInfos []Info, total int) (map[string]int, error) {
	deployMethod, ok := Plans[strategy]
	if !ok {
		return nil, types.ErrInvaildDeployStrategy
	}
	if count <= 0 {
		return nil, types.ErrInvaildDeployCount
	}

	log.WithFunc("strategy.Deploy").Debugf(ctx, "strategy %s, infos %+v, need %d, total %d, limit %d", strategy, strategyInfos, count, total, nodesLimit)
	return deployMethod(ctx, strategyInfos, count, total, nodesLimit)
}
