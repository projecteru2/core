package etcdv3

import (
	"strings"

	"github.com/projecteru2/core/types"
)

func parseStatusKey(key string) (string, string, string, string) {
	parts := strings.Split(key, "/")
	l := len(parts)
	return parts[l-4], parts[l-3], parts[l-2], parts[l-1]
}

func setCount(nodesCount map[string]int, nodesInfo []types.NodeInfo) []types.NodeInfo {
	for p, nodeInfo := range nodesInfo {
		if v, ok := nodesCount[nodeInfo.Name]; ok {
			nodesInfo[p].Count += v
		}
	}
	return nodesInfo
}
