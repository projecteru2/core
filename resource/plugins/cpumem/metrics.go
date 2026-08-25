package cpumem

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-viper/mapstructure/v2"

	plugintypes "github.com/projecteru2/core/resource/plugins/types"
)

const (
	fieldName   = "name"
	fieldHelp   = "help"
	fieldType   = "type"
	fieldLabels = "labels"
	fieldValue  = "value"
	fieldKey    = "key"

	gaugeType = "gauge"

	labelPodname  = "podname"
	labelNodename = "nodename"
	labelCPUID    = "cpuid"
)

func (p Plugin) GetMetricsDescription(context.Context) (*plugintypes.GetMetricsDescriptionResponse, error) {
	resp := &plugintypes.GetMetricsDescriptionResponse{}
	return resp, mapstructure.Decode([]map[string]any{
		{
			fieldName:   "cpu_map",
			fieldHelp:   "node available cpu.",
			fieldType:   gaugeType,
			fieldLabels: []string{labelPodname, labelNodename, labelCPUID},
		},
		{
			fieldName:   "cpu_used",
			fieldHelp:   "node used cpu.",
			fieldType:   gaugeType,
			fieldLabels: []string{labelPodname, labelNodename},
		},
		{
			fieldName:   "memory_capacity",
			fieldHelp:   "node available memory.",
			fieldType:   gaugeType,
			fieldLabels: []string{labelPodname, labelNodename},
		},
		{
			fieldName:   "memory_used",
			fieldHelp:   "node used memory.",
			fieldType:   gaugeType,
			fieldLabels: []string{labelPodname, labelNodename},
		},
	}, resp)
}

func (p Plugin) GetMetrics(ctx context.Context, podname, nodename string) (*plugintypes.GetMetricsResponse, error) {
	nodeResourceInfo, err := p.doGetNodeResourceInfo(ctx, nodename)
	if err != nil {
		return nil, err
	}
	safeNodename := strings.ReplaceAll(nodename, ".", "_")
	metrics := []map[string]any{
		{
			fieldName:   "memory_capacity",
			fieldLabels: []string{podname, nodename},
			fieldValue:  fmt.Sprintf("%+v", nodeResourceInfo.Capacity.Memory),
			fieldKey:    fmt.Sprintf("core.node.%s.memory", safeNodename),
		},
		{
			fieldName:   "memory_used",
			fieldLabels: []string{podname, nodename},
			fieldValue:  fmt.Sprintf("%+v", nodeResourceInfo.Usage.Memory),
			fieldKey:    fmt.Sprintf("core.node.%s.memory.used", safeNodename),
		},
		{
			fieldName:   "cpu_used",
			fieldLabels: []string{podname, nodename},
			fieldValue:  fmt.Sprintf("%+v", nodeResourceInfo.Usage.CPU),
			fieldKey:    fmt.Sprintf("core.node.%s.cpu.used", safeNodename),
		},
	}

	for cpuID, pieces := range nodeResourceInfo.Usage.CPUMap {
		metrics = append(metrics, map[string]any{
			fieldName:   "cpu_map",
			fieldLabels: []string{podname, nodename, cpuID},
			fieldValue:  fmt.Sprintf("%+v", pieces),
			fieldKey:    fmt.Sprintf("core.node.%s.cpu.%s", safeNodename, cpuID),
		})
	}

	resp := &plugintypes.GetMetricsResponse{}
	return resp, mapstructure.Decode(metrics, resp)
}
