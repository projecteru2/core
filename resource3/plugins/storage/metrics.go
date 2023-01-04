package storage

import (
	"context"
	"fmt"
	"strings"

	"github.com/mitchellh/mapstructure"
	plugintypes "github.com/projecteru2/core/resource3/plugins/types"
)

// GetMetricsDescription .
func (p Plugin) GetMetricsDescription(ctx context.Context) (*plugintypes.GetMetricsDescriptionResponse, error) {
	resp := &plugintypes.GetMetricsDescriptionResponse{}
	return resp, mapstructure.Decode([]map[string]interface{}{
		{
			"name":   "storage_used",
			"help":   "node used storage.",
			"type":   "gauge",
			"labels": []string{"podname", "nodename"},
		},
		{
			"name":   "storage_capacity",
			"help":   "node available storage.",
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
	metrics := []map[string]interface{}{
		{
			"name":   "storage_used",
			"labels": []string{podname, nodename},
			"value":  fmt.Sprintf("%+v", nodeResourceInfo.Usage.Storage),
			"key":    fmt.Sprintf("core.node.%s.storage.used", safeNodename),
		},
		{
			"name":   "storage_capacity",
			"labels": []string{podname, nodename},
			"value":  fmt.Sprintf("%+v", nodeResourceInfo.Capacity.Storage),
			"key":    fmt.Sprintf("core.node.%s.storage.used", safeNodename),
		},
	}

	resp := &plugintypes.GetMetricsResponse{}
	return resp, mapstructure.Decode(metrics, resp)
}
