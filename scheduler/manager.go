package scheduler

import (
	"math"

	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

func SelectNodes(resourceRequests []types.ResourceRequest, nodesInfo []types.NodeInfo) (planMap map[types.ResourceType]types.ResourcePlans, total int, err error) {
	total = math.MaxInt16
	subTotal := 0
	planMap = make(map[types.ResourceType]types.ResourcePlans)
	for _, req := range resourceRequests {
		scheduler := req.MakeScheduler()
		if planMap[req.Type()], subTotal, err = scheduler(nodesInfo); err != nil {
			return
		}
		total = utils.Min(total, subTotal)
	}
	return
}
