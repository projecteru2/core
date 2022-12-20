package types

// GetMetricsDescriptionRequest .
type GetMetricsDescriptionRequest struct{}

// GetMetricsRequest .
type GetMetricsRequest struct {
	Podname  string `json:"podname" mapstructure:"podname"`
	Nodename string `json:"nodename" mapstructure:"nodename"`
}
