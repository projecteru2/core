package models

import (
	"fmt"
	"strings"

	"github.com/projecteru2/core/resources/cpumem/types"
)

// GetMetricsDescription .
func (c *CPUMem) GetMetricsDescription() []map[string]interface{} {
	return []map[string]interface{}{
		{
			"name":   "memory_capacity",
			"help":   "node available memory.",
			"type":   "gauge",
			"labels": []string{"podname", "nodename"},
		},
		{
			"name":   "memory_used",
			"help":   "node used memory.",
			"type":   "gauge",
			"labels": []string{"podname", "nodename"},
		},
		{
			"name":   "cpu_map",
			"help":   "node available cpu.",
			"type":   "gauge",
			"labels": []string{"podname", "nodename", "cpuid"},
		},
		{
			"name":   "cpu_used",
			"help":   "node used cpu.",
			"type":   "gauge",
			"labels": []string{"podname", "nodename"},
		},
	}
}

func (c *CPUMem) GetNodeMetrics(podname string, nodename string, nodeResourceCapacity *types.NodeResourceArgs, nodeResourceUsage *types.NodeResourceArgs) []map[string]interface{} {
	cleanedNodeName := strings.ReplaceAll(nodename, ".", "_")
	metrics := []map[string]interface{}{
		{
			"name":   "memory_capacity",
			"labels": []string{podname, nodename},
			"value":  fmt.Sprintf("%+v", nodeResourceCapacity.Memory),
			"key":    fmt.Sprintf("core.node.%s.memory", cleanedNodeName),
		},
		{
			"name":   "memory_used",
			"labels": []string{podname, nodename},
			"value":  fmt.Sprintf("%+v", nodeResourceUsage.Memory),
			"key":    fmt.Sprintf("core.node.%s.memory.used", cleanedNodeName),
		},
		{
			"name":   "cpu_used",
			"labels": []string{podname, nodename},
			"value":  fmt.Sprintf("%+v", nodeResourceUsage.CPU),
			"key":    fmt.Sprintf("core.node.%s.cpu.used", cleanedNodeName),
		},
	}

	for cpuID, pieces := range nodeResourceUsage.CPUMap {
		metrics = append(metrics, map[string]interface{}{
			"name":   "cpu_map",
			"labels": []string{podname, nodename, cpuID},
			"value":  fmt.Sprintf("%+v", pieces),
			"key":    fmt.Sprintf("core.node.%s.cpu.%s", cleanedNodeName, cpuID),
		})
	}
	return metrics
}
