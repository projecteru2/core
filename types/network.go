package types

// Network for network define
type Network struct {
	Name    string   `json:"name"`
	Subnets []string `json:"cidr"`
}
