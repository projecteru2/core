package cpumem

import (
	"context"
	"fmt"
	"strings"

	cpumemtypes "github.com/projecteru2/core/resource/plugins/cpumem/types"
	plugintypes "github.com/projecteru2/core/resource/plugins/types"
	resourcetypes "github.com/projecteru2/core/resource/types"
	"github.com/projecteru2/core/utils"
)

const (
	fieldName   = "name"
	fieldHelp   = "help"
	fieldType   = "type"
	fieldLabels = "labels"

	gaugeType = "gauge"

	labelPodname  = "podname"
	labelNodename = "nodename"
	labelCPUID    = "cpuid"
)

func (p Plugin) GetMetricsDescription(context.Context) (*plugintypes.GetMetricsDescriptionResponse, error) {
	resp := &plugintypes.GetMetricsDescriptionResponse{}
	return resp, resourcetypes.Decode([]map[string]any{
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

func (p Plugin) GetMetrics(ctx context.Context, nodes []plugintypes.NodeRef) (*plugintypes.GetMetricsResponse, error) {
	infos, err := p.doGetNodesResourceInfo(ctx, utils.Map(nodes, func(node plugintypes.NodeRef) string { return node.Nodename }))
	if err != nil {
		return nil, err
	}
	metrics := plugintypes.GetMetricsResponse{}
	for _, node := range nodes {
		metrics = append(metrics, nodeMetrics(node, infos[node.Nodename])...)
	}
	return &metrics, nil
}

func nodeMetrics(node plugintypes.NodeRef, info *cpumemtypes.NodeResourceInfo) []*plugintypes.Metrics {
	labels := []string{node.Podname, node.Nodename}
	safeNodename := strings.ReplaceAll(node.Nodename, ".", "_")
	metrics := []*plugintypes.Metrics{
		{Name: "memory_capacity", Labels: labels, Value: fmt.Sprintf("%+v", info.Capacity.Memory), Key: fmt.Sprintf("core.node.%s.memory", safeNodename)},
		{Name: "memory_used", Labels: labels, Value: fmt.Sprintf("%+v", info.Usage.Memory), Key: fmt.Sprintf("core.node.%s.memory.used", safeNodename)},
		{Name: "cpu_used", Labels: labels, Value: fmt.Sprintf("%+v", info.Usage.CPU), Key: fmt.Sprintf("core.node.%s.cpu.used", safeNodename)},
	}
	for cpuID, pieces := range info.Usage.CPUMap {
		metrics = append(metrics, &plugintypes.Metrics{
			Name:   "cpu_map",
			Labels: []string{node.Podname, node.Nodename, cpuID},
			Value:  fmt.Sprintf("%+v", pieces),
			Key:    fmt.Sprintf("core.node.%s.cpu.%s", safeNodename, cpuID),
		})
	}
	return metrics
}
