package containerd

import (
	"context"
	"encoding/json"
	"io"
	"slices"
	"strings"

	"github.com/cockroachdb/errors"

	"github.com/projecteru2/core/engine/sshrunner"
	enginetypes "github.com/projecteru2/core/engine/types"
	coretypes "github.com/projecteru2/core/types"
)

const (
	// cniConfDir is where the CNI plugins the eru hook drives read their configuration.
	cniConfDir = "/etc/cni/net.d"

	listNetworkScript = `cat "$1"/*.conflist "$1"/*.conf 2>/dev/null || true`
)

// cniConf is the subset of a CNI network configuration core reports.
type cniConf struct {
	Name    string    `json:"name"`
	Type    string    `json:"type"`
	IPAM    cniIPAM   `json:"ipam"`
	Plugins []cniConf `json:"plugins"`
}

func (c cniConf) drives(drivers []string) bool {
	if len(drivers) == 0 {
		return true
	}
	if slices.Contains(drivers, c.Type) {
		return true
	}
	return slices.ContainsFunc(c.Plugins, func(plugin cniConf) bool {
		return slices.Contains(drivers, plugin.Type)
	})
}

func (c cniConf) subnets() []string {
	subnets := c.IPAM.subnets()
	for _, plugin := range c.Plugins {
		subnets = slices.Concat(subnets, plugin.IPAM.subnets())
	}
	return subnets
}

type cniIPAM struct {
	Subnet string `json:"subnet"`
	Ranges [][]struct {
		Subnet string `json:"subnet"`
	} `json:"ranges"`
}

func (i cniIPAM) subnets() []string {
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

// NetworkConnect is not a CNI operation: a network is attached when the netns is created.
func (e *Engine) NetworkConnect(context.Context, string, string, string, string) ([]string, error) {
	return nil, coretypes.ErrEngineNotImplemented
}

func (e *Engine) NetworkDisconnect(context.Context, string, string, bool) error {
	return coretypes.ErrEngineNotImplemented
}

func (e *Engine) NetworkList(ctx context.Context, drivers []string) ([]*enginetypes.Network, error) {
	res, err := e.run(ctx, sshrunner.Shell(listNetworkScript, cniConfDir)...)
	if err != nil {
		return nil, err
	}
	networks := []*enginetypes.Network{}
	decoder := json.NewDecoder(strings.NewReader(res.Stdout))
	for {
		conf := cniConf{}
		if decodeErr := decoder.Decode(&conf); decodeErr != nil {
			if errors.Is(decodeErr, io.EOF) {
				return networks, nil
			}
			return nil, decodeErr
		}
		if conf.Name == "" || !conf.drives(drivers) {
			continue
		}
		networks = append(networks, &enginetypes.Network{Name: conf.Name, Subnets: conf.subnets()})
	}
}
