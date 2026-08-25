package types

type MetricsDescription struct {
	Name   string   `json:"name" mapstructure:"name"`
	Help   string   `json:"help" mapstructure:"help"`
	Type   string   `json:"type" mapstructure:"type"`
	Labels []string `json:"labels" mapstructure:"labels"`
}

type GetMetricsDescriptionResponse []*MetricsDescription

type Metrics struct {
	Name   string   `json:"name" mapstructure:"name"`
	Labels []string `json:"labels" mapstructure:"labels"`
	Key    string   `json:"key" mapstructure:"key"`
	Value  string   `json:"value" mapstructure:"value"`
}

type GetMetricsResponse []*Metrics
