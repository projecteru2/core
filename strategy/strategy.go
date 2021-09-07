package strategy

import (
	"context"
	"math"

	"github.com/projecteru2/core/log"
	resourcetypes "github.com/projecteru2/core/resources/types"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"

	"github.com/pkg/errors"
)

const (
	// Auto .
	Auto = "AUTO"
	// Fill .
	Fill = "FILL"
	// Each .
	Each = "EACH"
	// Global .
	Global = "GLOBAL"
	// Dummy for calculate capacity
	Dummy = "DUMMY"
)

// Plans .
var Plans = map[string]strategyFunc{
	Auto:   CommunismPlan,
	Fill:   FillPlan,
	Each:   AveragePlan,
	Global: GlobalPlan,
}

type strategyFunc = func(_ context.Context, _ []Info, need, total, limit int) (map[string]int, error)

// Deploy .
func Deploy(ctx context.Context, opts *types.DeployOptions, strategyInfos []Info, total int) (map[string]int, error) {
	deployMethod, ok := Plans[opts.DeployStrategy]
	if !ok {
		return nil, errors.WithStack(types.ErrBadDeployStrategy)
	}
	if opts.Count <= 0 {
		return nil, errors.WithStack(types.ErrBadCount)
	}

	log.Debugf(ctx, "[strategy.Deploy] infos %+v, need %d, total %d, limit %d", strategyInfos, opts.Count, total, opts.NodesLimit)
	return deployMethod(ctx, strategyInfos, opts.Count, total, opts.NodesLimit)
}

// Info .
type Info struct {
	Nodename string

	Usage float64
	Rate  float64

	Capacity int
	Count    int
}

// NewInfos .
// TODO strange name, need to revise
func NewInfos(resourceRequests resourcetypes.ResourceRequests, nodeMap map[string]*types.Node, plans []resourcetypes.ResourcePlans) (strategyInfos []Info) {
	for nodename, node := range nodeMap {
		capacity := math.MaxInt64
		for _, plan := range plans {
			capacity = utils.Min(capacity, plan.Capacity()[nodename])
		}
		if capacity <= 0 {
			continue
		}

		strategyInfos = append(strategyInfos, Info{
			Nodename: nodename,
			Rate:     resourceRequests.MainRateOnNode(*node),
			Usage:    resourceRequests.MainUsageOnNode(*node),
			Capacity: capacity,
		})
	}
	return
}
