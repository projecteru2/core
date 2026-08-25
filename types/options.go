package types

import (
	"fmt"
	"io"
	"slices"

	resourcetypes "github.com/projecteru2/core/resource/types"
)

const (
	TriKeep TriOptions = iota
	TriTrue
	TriFalse

	SendLargeFileChunkSize = 2 << 10
)

// Processing tracks the unfinished workload count for one deploy.
type Processing struct {
	Appname   string
	Entryname string
	Nodename  string
	Ident     string
}

type DeployOptions struct {
	Resources      resourcetypes.Resources
	Name           string
	Entrypoint     *Entrypoint
	Podname        string
	NodeFilter     *NodeFilter
	Image          string
	ExtraArgs      string // appended to the entrypoint command
	Count          int
	Env            []string
	DNS            []string
	ExtraHosts     []string
	Networks       map[string]string // network name to specified IP
	User           string
	Debug          bool // use syslog as log driver
	OpenStdin      bool
	Labels         map[string]string
	DeployStrategy string
	Files          []LinuxFile
	NodesLimit     int
	ProcessIdent   string
	IgnoreHook     bool
	AfterCreate    []string
	RawArgs        RawArgs
	Lambda         bool
	IgnorePull     bool
}

func (o DeployOptions) GetProcessing(nodename string) *Processing {
	return &Processing{
		Appname:   o.Name,
		Entryname: o.Entrypoint.Name,
		Nodename:  nodename,
		Ident:     o.ProcessIdent,
	}
}

func (o *DeployOptions) Validate() error {
	if o.Name == "" {
		return ErrEmptyAppName
	}
	if o.Podname == "" {
		return ErrEmptyPodName
	}
	if o.Image == "" {
		return ErrEmptyImage
	}
	if o.Count == 0 {
		return ErrEmptyCount
	}
	return o.Entrypoint.Validate()
}

type CopyOptions struct {
	Targets map[string][]string
}

func (o *CopyOptions) Validate() error {
	if len(o.Targets) == 0 {
		return ErrNoFilesToCopy
	}
	return nil
}

type LinuxFile struct {
	Content  []byte
	Filename string
	UID      int
	GID      int
	Mode     int64
}

// Clone deep-copies Content.
func (f LinuxFile) Clone() LinuxFile {
	return LinuxFile{
		Content:  slices.Clone(f.Content),
		Filename: f.Filename,
		UID:      f.UID,
		GID:      f.GID,
		Mode:     f.Mode,
	}
}

func (f LinuxFile) String() string {
	return fmt.Sprintf("file %+v:%+v:%+v:%#o, len: %+v", f.Filename, f.UID, f.GID, f.Mode, len(f.Content))
}

// LitterDump renders the file for litter.Sdump.
func (f LinuxFile) LitterDump(w io.Writer) {
	_, _ = fmt.Fprintf(w, `{Content:{%d bytes},Filename:%s,UID:%d,GID:%d,Mode:%#o"}`, len(f.Content), f.Filename, f.UID, f.GID, f.Mode)
}

type SendOptions struct {
	IDs   []string
	Files []LinuxFile
}

func (o *SendOptions) Validate() error {
	if len(o.IDs) == 0 {
		return ErrNoWorkloadIDs
	}
	if len(o.Files) == 0 {
		return ErrNoFilesToSend
	}
	for i, file := range o.Files {
		if file.UID == 0 && file.GID == 0 && file.Mode == 0 {
			o.Files[i].Mode = 0o755
		}
	}
	return nil
}

type ListWorkloadsOptions struct {
	Appname    string
	Entrypoint string
	Nodename   string
	Limit      int64
	Labels     map[string]string
}

type ReplaceOptions struct {
	DeployOptions
	NetworkInherit bool
	FilterLabels   map[string]string
	Copy           map[string]string
	IDs            []string
}

// Validate skips Image; pullImage in cluster/calcium checks it.
func (o *ReplaceOptions) Validate() error {
	if o.Name == "" {
		return ErrEmptyAppName
	}
	return o.Entrypoint.Validate()
}

// Normalize defaults Count to 1.
func (o *ReplaceOptions) Normalize() {
	if o.Count == 0 {
		o.Count = 1
	}
}

type ListNodesOptions struct {
	Podname  string
	Labels   map[string]string
	All      bool
	CallInfo bool
}

type AddNodeOptions struct {
	Nodename  string
	Endpoint  string
	Podname   string
	Ca        string
	Cert      string
	Key       string
	Labels    map[string]string
	Resources resourcetypes.Resources
	Test      bool
}

func (o *AddNodeOptions) Validate() error {
	if o.Nodename == "" {
		return ErrEmptyNodeName
	}
	if o.Podname == "" {
		return ErrEmptyPodName
	}
	if o.Endpoint == "" {
		return ErrInvaildNodeEndpoint
	}
	return nil
}

type SetNodeOptions struct {
	Nodename      string
	Endpoint      string
	WorkloadsDown bool
	Resources     resourcetypes.Resources
	Delta         bool
	Labels        map[string]string
	Bypass        TriOptions
	Ca            string
	Cert          string
	Key           string
}

func (o *SetNodeOptions) Validate() error {
	if o.Nodename == "" {
		return ErrEmptyNodeName
	}
	return nil
}

// ImageOptions carries image op options; Prune applies to remove only.
type ImageOptions struct {
	Podname   string
	Nodenames []string
	Images    []string
	Prune     bool
	Filter    string
}

func (o *ImageOptions) Validate() error {
	if o.Podname == "" {
		return ErrEmptyPodName
	}
	return nil
}

type ExecuteWorkloadOptions struct {
	WorkloadID string
	Commands   []string
	Envs       []string
	Workdir    string
	OpenStdin  bool
	ReplCmd    []byte
}

type ReallocOptions struct {
	ID        string
	Resources resourcetypes.Resources
}

type TriOptions int

type RawArgs []byte

func (r RawArgs) String() string {
	return string(r)
}

// LitterDump renders the raw args for litter.Sdump.
func (r RawArgs) LitterDump(w io.Writer) {
	_, _ = w.Write(r)
}

// SendLargeFileOptions carries one chunk of a SendLargeFile stream.
type SendLargeFileOptions struct {
	IDs   []string
	Dst   string
	Size  int64
	Mode  int64
	UID   int
	GID   int
	Chunk []byte
}

func (o *SendLargeFileOptions) Validate() error {
	if len(o.IDs) == 0 {
		return ErrNoWorkloadIDs
	}
	if len(o.Chunk) == 0 {
		return ErrNoFilesToSend
	}
	if o.UID == 0 && o.GID == 0 && o.Mode == 0 {
		o.Mode = 0o755
	}
	return nil
}

type RawEngineOptions struct {
	ID         string
	Op         string
	Params     []byte
	IgnoreLock bool
}

func (o *RawEngineOptions) Validate() error {
	if o.ID == "" {
		return ErrEmptyWorkloadID
	}
	if o.Op == "" {
		return ErrEmptyRawEngineOp
	}
	return nil
}
