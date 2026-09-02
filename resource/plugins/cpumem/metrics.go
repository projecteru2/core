package cpumem

import (
	"context"
	"fmt"
	"strings"

	cpumemtypes "github.com/projecteru2/core/resource/plugins/cpumem/types"
	plugintypes "github.com/projecteru2/core/resource/plugins/types"
	"github.com/projecteru2/core/utils"
)

const (
	gaugeType = "gauge"

	labelPodname  = "podname"
	labelNodename = "nodename"
	labelCPUID    = "cpuid"
)

func (p Plugin) GetMetricsDescription(context.Context) (*plugintypes.GetMetricsDescriptionResponse, error) {
	return &plugintypes.GetMetricsDescriptionResponse{
		{
			Name:   "cpu_map",
			Help:   "node available cpu.",
			Type:   gaugeType,
			Labels: []string{labelPodname, labelNodename, labelCPUID},
		},
		{
			Name:   "cpu_used",
			Help:   "node used cpu.",
			Type:   gaugeType,
			Labels: []string{labelPodname, labelNodename},
		},
		{
			Name:   "memory_capacity",
			Help:   "node available memory.",
			Type:   gaugeType,
			Labels: []string{labelPodname, labelNodename},
		},
		{
			Name:   "memory_used",
			Help:   "node used memory.",
			Type:   gaugeType,
			Labels: []string{labelPodname, labelNodename},
		},
	}, nil
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
