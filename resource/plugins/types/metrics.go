package types

// NodeRef names one node in a metrics request.
type NodeRef struct {
	Podname  string `json:"podname"`
	Nodename string `json:"nodename"`
}

type MetricsDescription struct {
	Name   string   `json:"name"`
	Help   string   `json:"help"`
	Type   string   `json:"type"`
	Labels []string `json:"labels"`
}

type GetMetricsDescriptionResponse []*MetricsDescription

type Metrics struct {
	Name   string   `json:"name"`
	Labels []string `json:"labels"`
	Key    string   `json:"key"`
	Value  string   `json:"value"`
}

type GetMetricsResponse []*Metrics
