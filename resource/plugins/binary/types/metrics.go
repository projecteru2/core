package types

type GetMetricsDescriptionRequest struct{}

type GetMetricsRequest struct {
	Podname  string `json:"podname"`
	Nodename string `json:"nodename"`
}
