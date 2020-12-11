package strategy

import (
	"fmt"
	"sort"

	"github.com/pkg/errors"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
)

// CommunismPlan 吃我一记共产主义大锅饭
// 部署完 N 个后全局尽可能平均
func CommunismPlan(arg []Info, need, total, limit int, resourceType types.ResourceType) (map[string]int, error) {
	if total < need {
		return nil, errors.WithStack(types.NewDetailedErr(types.ErrInsufficientRes,
			fmt.Sprintf("need: %d, vol: %d", need, total)))
	}
	sort.Slice(arg, func(i, j int) bool { return arg[i].Count < arg[j].Count })
	length := len(arg)

	deployMap := map[string]int{}
	for i := 0; i < length; i++ {
		if need <= 0 {
			break
		}
		req := need
		if i < length-1 {
			req = (arg[i+1].Count - arg[i].Count) * (i + 1)
		}
		if req > need {
			req = need
		}
		for j := 0; j < i+1; j++ {
			deploy := req / (i + 1 - j)
			tail := req % (i + 1 - j)
			d := deploy
			if tail > 0 {
				d++
			}
			if d > arg[j].Capacity {
				d = arg[j].Capacity
			}
			if d == 0 {
				continue
			}
			arg[j].Capacity -= d
			deployMap[arg[j].Nodename] += d
			need -= d
			req -= d
		}
	}
	// 这里 need 一定会为 0 出来，因为 volTotal 保证了一定大于 need
	// 这里并不需要再次排序了，理论上的排序是基于 Count 得到的 Deploy 最终方案
	log.Debugf("[CommunismPlan] strategyInfo: %+v", arg)
	return deployMap, nil
}
