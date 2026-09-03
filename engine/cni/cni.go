// Package cni lists the networks a node's CNI conf dir declares.
package cni

import (
	"encoding/json"
	"io"
	"slices"
	"strings"

	"github.com/cockroachdb/errors"

	enginetypes "github.com/projecteru2/core/engine/types"
)

const (
	// ConfDir is where the CNI plugins read their configuration.
	ConfDir = "/etc/cni/net.d"
	// ListScript prints every conf under the dir given as $1, in the shape Parse reads.
	ListScript = `cat "$1"/*.conflist "$1"/*.conf 2>/dev/null || true`
)

// Conf is the subset of a CNI network configuration core reports.
type Conf struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	IPAM    ipam   `json:"ipam"`
	Plugins []Conf `json:"plugins"`
}

// Subnets lists what the conf and its plugins hand addresses out of.
func (c Conf) Subnets() []string {
	subnets := c.IPAM.subnets()
	for _, plugin := range c.Plugins {
		subnets = slices.Concat(subnets, plugin.IPAM.subnets())
	}
	return subnets
}

type ipam struct {
	Subnet string `json:"subnet"`
	Ranges [][]struct {
		Subnet string `json:"subnet"`
	} `json:"ranges"`
}

func (i ipam) subnets() []string {
	subnets := []string{}
	if i.Subnet != "" {
		subnets = append(subnets, i.Subnet)
	}
	for _, set := range i.Ranges {
		for _, entry := range set {
			if entry.Subnet != "" {
				subnets = append(subnets, entry.Subnet)
			}
		}
	}
	return subnets
}

// Parse turns the confs ListScript printed into networks, narrowed to drivers when any are named.
func Parse(confs string, drivers []string) ([]*enginetypes.Network, error) {
	networks := []*enginetypes.Network{}
	decoder := json.NewDecoder(strings.NewReader(confs))
	for {
		c := Conf{}
		if err := decoder.Decode(&c); err != nil {
			if errors.Is(err, io.EOF) {
				return networks, nil
			}
			return nil, err
		}
		if c.Name == "" || !drives(c, drivers) {
			continue
		}
		networks = append(networks, &enginetypes.Network{Name: c.Name, Subnets: c.Subnets()})
	}
}

// drives reports whether one of the named plugin types implements the conf.
func drives(c Conf, drivers []string) bool {
	if len(drivers) == 0 {
		return true
	}
	if slices.Contains(drivers, c.Type) {
		return true
	}
	return slices.ContainsFunc(c.Plugins, func(plugin Conf) bool {
		return slices.Contains(drivers, plugin.Type)
	})
}
