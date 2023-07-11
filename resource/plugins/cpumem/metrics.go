package cpumem

import (
	"context"
	"fmt"
	"strings"

	"github.com/mitchellh/mapstructure"
	plugintypes "github.com/projecteru2/core/resource/plugins/types"
)

// GetMetricsDescription .
func (p Plugin) GetMetricsDescription(context.Context) (*plugintypes.GetMetricsDescriptionResponse, error) {
	resp := &plugintypes.GetMetricsDescriptionResponse{}
	return resp, mapstructure.Decode([]map[string]any{
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
	}, resp)
}

// GetMetrics .
func (p Plugin) GetMetrics(ctx context.Context, podname, nodename string) (*plugintypes.GetMetricsResponse, error) {
	nodeResourceInfo, err := p.doGetNodeResourceInfo(ctx, nodename)
	if err != nil {
		return nil, err
	}
	safeNodename := strings.ReplaceAll(nodename, ".", "_")
	metrics := []map[string]any{
		{
			"name":   "memory_capacity",
			"labels": []string{podname, nodename},
			"value":  fmt.Sprintf("%+v", nodeResourceInfo.Capacity.Memory),
			"key":    fmt.Sprintf("core.node.%s.memory", safeNodename),
		},
		{
			"name":   "memory_used",
			"labels": []string{podname, nodename},
			"value":  fmt.Sprintf("%+v", nodeResourceInfo.Usage.Memory),
			"key":    fmt.Sprintf("core.node.%s.memory.used", safeNodename),
		},
		{
			"name":   "cpu_used",
			"labels": []string{podname, nodename},
			"value":  fmt.Sprintf("%+v", nodeResourceInfo.Usage.CPU),
			"key":    fmt.Sprintf("core.node.%s.cpu.used", safeNodename),
		},
	}

	for cpuID, pieces := range nodeResourceInfo.Usage.CPUMap {
		metrics = append(metrics, map[string]any{
			"name":   "cpu_map",
			"labels": []string{podname, nodename, cpuID},
			"value":  fmt.Sprintf("%+v", pieces),
			"key":    fmt.Sprintf("core.node.%s.cpu.%s", safeNodename, cpuID),
		})
	}

	resp := &plugintypes.GetMetricsResponse{}
	return resp, mapstructure.Decode(metrics, resp)
}
