package etcdv3

import (
	"strings"

	"github.com/projecteru2/core/strategy"
)

func parseStatusKey(key string) (string, string, string, string) {
	parts := strings.Split(key, "/")
	l := len(parts)
	return parts[l-4], parts[l-3], parts[l-2], parts[l-1]
}

func setCount(nodesCount map[string]int, strategyInfos []strategy.Info) {
	for i, strategyInfo := range strategyInfos {
		if v, ok := nodesCount[strategyInfo.Nodename]; ok {
			strategyInfos[i].Count += v
		}
	}
}
