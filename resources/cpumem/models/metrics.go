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

func (c *CPUMem) ConvertNodeResourceInfoToMetrics(podName string, nodeName string, nodeResourceCapacity *types.NodeResourceArgs, nodeResourceUsage *types.NodeResourceArgs) []map[string]interface{} {
	cleanedNodeName := strings.ReplaceAll(nodeName, ".", "_")
	metrics := []map[string]interface{}{
		{
			"name":   "memory_capacity",
			"labels": []string{podName, nodeName},
			"value":  fmt.Sprintf("%v", nodeResourceCapacity.Memory),
			"key":    fmt.Sprintf("core.node.%s.memory", cleanedNodeName),
		},
		{
			"name":   "memory_used",
			"labels": []string{podName, nodeName},
			"value":  fmt.Sprintf("%v", nodeResourceUsage.Memory),
			"key":    fmt.Sprintf("core.node.%s.memory.used", cleanedNodeName),
		},
		{
			"name":   "cpu_used",
			"labels": []string{podName, nodeName},
			"value":  fmt.Sprintf("%v", nodeResourceUsage.CPU),
			"key":    fmt.Sprintf("core.node.%s.cpu.used", cleanedNodeName),
		},
	}

	for cpuID, pieces := range nodeResourceUsage.CPUMap {
		metrics = append(metrics, map[string]interface{}{
			"name":   "cpu_map",
			"labels": []string{podName, nodeName, cpuID},
			"value":  fmt.Sprintf("%v", pieces),
			"key":    fmt.Sprintf("core.node.%s.cpu.%s", cleanedNodeName, cpuID),
		})
	}
	return metrics
}
