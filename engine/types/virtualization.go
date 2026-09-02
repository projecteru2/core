package types

import (
	resourcetypes "github.com/projecteru2/core/resource/types"
)

// CPUPeriodBase is the cfs period a workload's cpu quota is expressed against.
const CPUPeriodBase = 100000

// VirtualizationCreateOptions describes a workload to create.
type VirtualizationCreateOptions struct {
	EngineParams resourcetypes.Resources
	Name         string
	User         string
	Image        string
	WorkingDir   string
	Stdin        bool
	Privileged   bool
	Cmd          []string
	Env          []string
	DNS          []string
	Hosts        []string
	Sysctl       map[string]string
	Labels       map[string]string

	Restart string

	Networks map[string]string

	RawArgs []byte
	Lambda  bool

	AncestorWorkloadID string
}

// VirtualizationCreated identifies a freshly created workload.
type VirtualizationCreated struct {
	ID     string
	Name   string
	Labels map[string]string
}

type VirtualizationInfo struct {
	ID       string
	User     string
	Image    string
	Running  bool
	Env      []string
	Labels   map[string]string
	Networks map[string]string
}

// VirtualizationWaitResult carries a workload's exit status.
type VirtualizationWaitResult struct {
	Message string
	Code    int64
}
