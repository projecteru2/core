package utils

import "github.com/projecteru2/core/types"

func EncodePorts(ports []string) []types.Port {
	p := []types.Port{}
	for _, port := range ports {
		p = append(p, types.Port(port))
	}
	return p
}

func DecodePorts(ports []types.Port) []string {
	p := []string{}
	for _, port := range ports {
		p = append(p, string(port))
	}
	return p
}
