package types

// Network is network info
type Network struct {
	Name    string   `json:"name"`
	Subnets []string `json:"cidr"`
}
