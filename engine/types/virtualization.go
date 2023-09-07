package types

import (
	resourcetypes "github.com/projecteru2/core/resource/types"
)

// VirtualizationCreateOptions use for create virtualization target
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
	Publish      []string
	Sysctl       map[string]string
	Labels       map[string]string

	Debug   bool
	Restart string

	Networks map[string]string

	LogType   string
	LogConfig map[string]string

	RawArgs []byte
	Lambda  bool

	AncestorWorkloadID string
}

// VirtualizationCreated use for store name and ID
type VirtualizationCreated struct {
	ID     string
	Name   string
	Labels map[string]string
}

// VirtualizationInfo store virtualization info
type VirtualizationInfo struct {
	ID       string
	User     string
	Image    string
	Running  bool
	Env      []string
	Labels   map[string]string
	Networks map[string]string
	// TODO other information like cpu memory
}

// VirtualizationWaitResult store exit result
type VirtualizationWaitResult struct {
	Message string
	Code    int64
}

// SendMessage returns from engine
type SendMessage struct {
	ID    string
	Path  string
	Error error
}
