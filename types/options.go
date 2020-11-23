package types

// DeployOptions is options for deploying
type DeployOptions struct {
	ResourceOpts   ResourceOptions
	Name           string                   // Name of application
	Entrypoint     *Entrypoint              // entrypoint
	Podname        string                   // Name of pod to deploy
	Nodenames      []string                 // Specific nodes to deploy, if given, must belong to pod
	Image          string                   // Name of image to deploy
	ExtraArgs      string                   // Extra arguments to append to command
	Count          int                      // How many workloads needed, e.g. 4
	Env            []string                 // Env for workload
	DNS            []string                 // DNS for workload
	ExtraHosts     []string                 // Extra hosts for workload
	Networks       map[string]string        // Network names and specified IPs
	NetworkMode    string                   // Network mode
	User           string                   // User for workload
	Debug          bool                     // debug mode, use syslog as log driver
	OpenStdin      bool                     // OpenStdin for workload
	Labels         map[string]string        // Labels for workloads
	NodeLabels     map[string]string        // NodeLabels for filter node
	DeployStrategy string                   // Deploy strategy
	Data           map[string]ReaderManager // For additional file data
	NodesLimit     int                      // Limit nodes count
	ProcessIdent   string                   // ProcessIdent ident this deploy
	IgnoreHook     bool                     // IgnoreHook ignore hook process
	AfterCreate    []string                 // AfterCreate support run cmds after create
	RawArgs        []byte                   // RawArgs for raw args processing
	Lambda         bool                     // indicate is lambda workload or not
}

// RunAndWaitOptions is options for running and waiting
type RunAndWaitOptions struct {
	DeployOptions
	Timeout int
	Cmd     string
}

// CopyOptions for multiple workload files copy
type CopyOptions struct {
	Targets map[string][]string
}

// SendOptions for send files to multiple workload
type SendOptions struct {
	IDs  []string
	Data map[string][]byte
}

// ListWorkloadsOptions for list workloads
type ListWorkloadsOptions struct {
	Appname    string
	Entrypoint string
	Nodename   string
	Limit      int64
	Labels     map[string]string
}

// ReplaceOptions for replace workload
type ReplaceOptions struct {
	DeployOptions
	NetworkInherit bool
	FilterLabels   map[string]string
	Copy           map[string]string
	IDs            []string
}

// AddNodeOptions for adding node
type AddNodeOptions struct {
	Nodename   string
	Endpoint   string
	Podname    string
	Ca         string
	Cert       string
	Key        string
	CPU        int
	Share      int
	Memory     int64
	Storage    int64
	Labels     map[string]string
	Numa       NUMA
	NumaMemory NUMAMemory
	Volume     VolumeMap
}

// Normalize keeps options consistent
func (o *AddNodeOptions) Normalize() {
	o.Storage += o.Volume.Total()
}

// SetNodeOptions for node set
type SetNodeOptions struct {
	Nodename        string
	StatusOpt       TriOptions
	WorkloadsDown   bool
	DeltaCPU        CPUMap
	DeltaMemory     int64
	DeltaStorage    int64
	DeltaNUMAMemory map[string]int64
	DeltaVolume     VolumeMap
	NUMA            map[string]string
	Labels          map[string]string
}

// Normalize keeps options consistent
func (o *SetNodeOptions) Normalize(node *Node) {
	o.DeltaStorage += o.DeltaVolume.Total()
	for volID, size := range o.DeltaVolume {
		if size == 0 {
			o.DeltaStorage -= node.InitVolume[volID]
		}
	}
}

// ExecuteWorkloadOptions for executing commands in running workload
type ExecuteWorkloadOptions struct {
	WorkloadID string
	Commands   []string
	Envs       []string
	Workdir    string
	OpenStdin  bool
	ReplCmd    []byte
}

// ReallocOptions .
type ReallocOptions struct {
	ID           string
	CPUBindOpts  TriOptions
	ResourceOpts ResourceOptions
}

// TriOptions .
type TriOptions int

const (
	// TriKeep .
	TriKeep = iota
	// TriTrue .
	TriTrue
	// TriFalse .
	TriFalse
)

// ParseTriOption .
func ParseTriOption(opt TriOptions, original bool) (res bool) {
	switch opt {
	case TriKeep:
		res = original
	case TriTrue:
		res = true
	case TriFalse:
		res = false
	}
	return
}
