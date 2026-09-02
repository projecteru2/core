package types

import plugintypes "github.com/projecteru2/core/resource/plugins/types"

type GetMetricsDescriptionRequest struct{}

type GetMetricsRequest struct {
	Nodes []plugintypes.NodeRef `json:"nodes"`
}
