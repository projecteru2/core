package types

// Pod define pod
type Pod struct {
	Name string `json:"name"`
	Desc string `json:"desc"`
}

// PodResource define pod resource
type PodResource struct {
	Name          string
	NodesResource []*NodeResource
}
