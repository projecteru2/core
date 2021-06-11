package types

import "github.com/pkg/errors"

// TODO should validate options

// DeployOptions is options for deploying
type DeployOptions struct {
	ResourceOpts   ResourceOptions
	Name           string                   // Name of application
	Entrypoint     *Entrypoint              // entrypoint
	Podname        string                   // Name of pod to deploy
	NodeFilter     NodeFilter               // filter of nodenames, using includes or not using excludes
	Image          string                   // Name of image to deploy
	ExtraArgs      string                   // Extra arguments to append to command
	Count          int                      // How many workloads needed, e.g. 4
	Env            []string                 // Env for workload
	DNS            []string                 // DNS for workload
	ExtraHosts     []string                 // Extra hosts for workload
	Networks       map[string]string        // Network names and specified IPs
	User           string                   // User for workload
	Debug          bool                     // debug mode, use syslog as log driver
	OpenStdin      bool                     // OpenStdin for workload
	Labels         map[string]string        // Labels for workloads
	DeployStrategy string                   // Deploy strategy
	Data           map[string]ReaderManager // For additional file data
	NodesLimit     int                      // Limit nodes count
	ProcessIdent   string                   // ProcessIdent ident this deploy
	IgnoreHook     bool                     // IgnoreHook ignore hook process
	AfterCreate    []string                 // AfterCreate support run cmds after create
	RawArgs        []byte                   // RawArgs for raw args processing
	Lambda         bool                     // indicate is lambda workload or not
}

// Processing tracks workloads count yet finished
type Processing struct {
	Appname   string
	Entryname string
	Nodename  string
	Ident     string
}

// GetProcessing .
func (o DeployOptions) GetProcessing(nodename string) *Processing {
	return &Processing{
		Appname:   o.Name,
		Entryname: o.Entrypoint.Name,
		Nodename:  nodename,
		Ident:     o.ProcessIdent,
	}
}

// Validate checks options
func (o *DeployOptions) Validate() error {
	if o.Name == "" {
		return errors.WithStack(ErrEmptyAppName)
	}
	if o.Podname == "" {
		return errors.WithStack(ErrEmptyPodName)
	}
	if o.Image == "" {
		return errors.WithStack(ErrEmptyImage)
	}
	if o.Count == 0 {
		return errors.WithStack(ErrEmptyCount)
	}
	return o.Entrypoint.Validate()
}

// CopyOptions for multiple workload files copy
type CopyOptions struct {
	Targets map[string][]string
}

// Validate checks options
func (o *CopyOptions) Validate() error {
	if len(o.Targets) == 0 {
		return errors.WithStack(ErrNoFilesToCopy)
	}
	return nil
}

// SendOptions for send files to multiple workload
type SendOptions struct {
	IDs  []string
	Data map[string][]byte
}

// Validate checks options
func (o *SendOptions) Validate() error {
	if len(o.IDs) == 0 {
		return errors.WithStack(ErrNoWorkloadIDs)
	}
	if len(o.Data) == 0 {
		return errors.WithStack(ErrNoFilesToSend)
	}
	return nil
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

// Validate doesn't check image here
// because in cluster/calcium//helper.go, pullImage will check this
// to keep the original behavior, no check here.
func (o *ReplaceOptions) Validate() error {
	if o.DeployOptions.Name == "" {
		return errors.WithStack(ErrEmptyAppName)
	}
	return o.DeployOptions.Entrypoint.Validate()
}

// Normalize checks count
func (o *ReplaceOptions) Normalize() {
	if o.Count == 0 {
		o.Count = 1
	}
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

// Validate checks options
func (o *AddNodeOptions) Validate() error {
	if o.Nodename == "" {
		return errors.WithStack(ErrEmptyNodeName)
	}
	if o.Podname == "" {
		return errors.WithStack(ErrEmptyPodName)
	}
	if o.Endpoint == "" {
		return errors.WithStack(ErrEmptyNodeEndpoint)
	}
	return nil
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

// Validate checks options
func (o *SetNodeOptions) Validate() error {
	if o.Nodename == "" {
		return errors.WithStack(ErrEmptyNodeName)
	}
	return nil
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

// ImageOptions wraps options for images
// Prune is only used when remove image
type ImageOptions struct {
	Podname   string
	Nodenames []string
	Images    []string
	Step      int
	Prune     bool
}

// Validate checks the options
func (o *ImageOptions) Validate() error {
	if o.Podname == "" {
		return errors.WithStack(ErrEmptyPodName)
	}
	return nil
}

// Normalize checks steps and set it properly
func (o *ImageOptions) Normalize() {
	if o.Step < 1 {
		o.Step = 1
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
