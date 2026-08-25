package types

type GetMetricsDescriptionRequest struct{}

type GetMetricsRequest struct {
	Podname  string `json:"podname" mapstructure:"podname"`
	Nodename string `json:"nodename" mapstructure:"nodename"`
}
