package types

import (
	"bytes"
	"io"
	"io/ioutil"
	"sync"
)

// DeployOptions is options for deploying
type DeployOptions struct {
	Name           string                   // Name of application
	Entrypoint     *Entrypoint              // entrypoint
	Podname        string                   // Name of pod to deploy
	Nodenames      []string                 // Specific nodes to deploy, if given, must belong to pod
	Image          string                   // Name of image to deploy
	ExtraArgs      string                   // Extra arguments to append to command
	CPUQuota       float64                  // How many cores needed, e.g. 1.5
	CPUBind        bool                     // Bind CPU or not ( old CPU piror )
	Memory         int64                    // Memory for container, in bytes
	Storage        int64                    // Storage for container, in bytes
	Count          int                      // How many containers needed, e.g. 4
	Env            []string                 // Env for container
	DNS            []string                 // DNS for container
	ExtraHosts     []string                 // Extra hosts for container
	Volumes        VolumeBindings           // Volumes for container
	Networks       map[string]string        // Network names and specified IPs
	NetworkMode    string                   // Network mode
	User           string                   // User for container
	Debug          bool                     // debug mode, use syslog as log driver
	OpenStdin      bool                     // OpenStdin for container
	Labels         map[string]string        // Labels for containers
	NodeLabels     map[string]string        // NodeLabels for filter node
	DeployStrategy string                   // Deploy strategy
	Data           map[string]ReaderManager // For additional file data
	SoftLimit      bool                     // Soft limit memory
	NodesLimit     int                      // Limit nodes count
	ProcessIdent   string                   // ProcessIdent ident this deploy
	IgnoreHook     bool                     // IgnoreHook ignore hook process
	AfterCreate    []string                 // AfterCreate support run cmds after create
	RawArgs        []byte                   // RawArgs for raw args processing
	Lambda         bool                     // indicate is lambda container or not
}

// ReaderManager return Reader under concurrency
type ReaderManager interface {
	GetReader() (io.Reader, error)
}

type readerManager struct {
	mux sync.Mutex
	r   io.ReadSeeker
}

func (rm *readerManager) GetReader() (_ io.Reader, err error) {
	rm.mux.Lock()
	defer rm.mux.Unlock()
	buf := &bytes.Buffer{}
	if _, err = io.Copy(buf, rm.r); err != nil {
		return
	}
	_, err = rm.r.Seek(0, io.SeekStart)
	return buf, err
}

// NewReaderManager converts Reader to ReadSeeker
func NewReaderManager(r io.Reader) (ReaderManager, error) {
	bs, err := ioutil.ReadAll(r)
	return &readerManager{
		r: bytes.NewReader(bs),
	}, err
}

// Normalize keeps deploy options consistent
func (o *DeployOptions) Normalize() {
	o.Storage += o.Volumes.TotalSize()
}

// RunAndWaitOptions is options for running and waiting
type RunAndWaitOptions struct {
	DeployOptions
	Timeout int
	Cmd     string
}

// CopyOptions for multiple container files copy
type CopyOptions struct {
	Targets map[string][]string
}

// SendOptions for send files to multiple container
type SendOptions struct {
	IDs  []string
	Data map[string][]byte
}

// ListContainersOptions for list containers
type ListContainersOptions struct {
	Appname    string
	Entrypoint string
	Nodename   string
	Limit      int64
	Labels     map[string]string
}

// ReplaceOptions for replace container
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
	Status          TriOptions
	ContainersDown  bool
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

// ExecuteContainerOptions for executing commands in running container
type ExecuteContainerOptions struct {
	ContainerID string
	Commands    []string
	Envs        []string
	Workdir     string
	OpenStdin   bool
	ReplCmd     []byte
}

// ReallocOptions .
type ReallocOptions struct {
	IDs         []string
	CPU         float64
	Memory      int64
	Volumes     VolumeBindings
	BindCPU     TriOptions
	MemoryLimit TriOptions
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

// CalculateCapacityOptions for CalculateCapacity API input
type CalculateCapacityOptions struct {
	Appname   string
	Entryname string
	Podname   string
	Nodenames []string
	CPUQuota  float64
	CPUBind   bool
	Memory    int64
	Volumes   VolumeBindings
	Storage   int64
}
