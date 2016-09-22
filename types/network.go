package types

type Network struct {
	Name    string   `json:"name"`
	Subnets []string `json:"cidr"`
}
