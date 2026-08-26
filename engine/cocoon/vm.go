package cocoon

import (
	"cmp"
	"encoding/json"
	"net"
	"path/filepath"
	"strings"
)

const (
	stateRunning    = "running"
	defaultNetwork  = "default"
	cloudHypervisor = "cloud-hypervisor"
	consoleSock     = "console.sock"
	ipv4Bits        = 32
)

type vmConfig struct {
	Image   string `json:"image"`
	Network string `json:"network"`
	Windows bool   `json:"windows"`
}

type nic struct {
	TAP     string        `json:"tap"`
	Network *guestAddress `json:"network"`
}

type guestAddress struct {
	IP      string `json:"ip"`
	Gateway string `json:"gateway"`
	Prefix  int    `json:"prefix"`
}

// mask renders the prefix the way netsh takes it.
func (a *guestAddress) mask() string {
	return net.IP(net.CIDRMask(a.Prefix, ipv4Bits)).String()
}

// vmRecord is the part of cocoon's VM JSON the engine reads.
type vmRecord struct {
	ID          string   `json:"id"`
	Hypervisor  string   `json:"hypervisor"`
	State       string   `json:"state"`
	FirstBooted bool     `json:"first_booted"`
	PID         int      `json:"pid"`
	ConsolePath string   `json:"console_path"`
	Config      vmConfig `json:"config"`
	NICs        []nic    `json:"network_configs"`
}

func parseVM(out string) (*vmRecord, error) {
	vm := &vmRecord{}
	if err := json.Unmarshal([]byte(out), vm); err != nil {
		return nil, err
	}
	return vm, nil
}

func parseVMs(out string) (*vmRecord, *vmRecord, error) {
	return decodePair[vmRecord, vmRecord](out)
}

func (v *vmRecord) running() bool {
	return v.State == stateRunning
}

// runDir is the VM's dir under cocoon's run dir, named after the backend without its hyphen.
func (v *vmRecord) runDir(base string) string {
	return filepath.Join(base, strings.ReplaceAll(cmp.Or(v.Hypervisor, cloudHypervisor), "-", ""), v.ID)
}

// console is the guest console cocoon resolved at this boot, the serial socket when it reports none.
func (v *vmRecord) console(base string) string {
	return cmp.Or(v.ConsolePath, filepath.Join(v.runDir(base), consoleSock))
}

// address is the guest's CNI address, nil on a DHCP network.
func (v *vmRecord) address() *guestAddress {
	for _, n := range v.NICs {
		if n.Network != nil && n.Network.IP != "" {
			return n.Network
		}
	}
	return nil
}

func (v *vmRecord) tap() string {
	if len(v.NICs) == 0 {
		return ""
	}
	return v.NICs[0].TAP
}

// networks keys the guest address by the conflist cocoon reports, which a deploy need not have named.
func (v *vmRecord) networks() map[string]string {
	addr := v.address()
	if addr == nil {
		return nil
	}
	return map[string]string{cmp.Or(v.Config.Network, defaultNetwork): addr.IP}
}

// vmEvent is one line of `vm status --event --format json`.
type vmEvent struct {
	Event string   `json:"event"`
	VM    vmRecord `json:"vm"`
}
