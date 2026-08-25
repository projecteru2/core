package types

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
