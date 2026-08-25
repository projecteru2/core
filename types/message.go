package types

import (
	"bytes"

	resourcetypes "github.com/projecteru2/core/resource/types"
)

type StdStreamType int

const (
	// EruError carries an eru error, not user program output.
	EruError StdStreamType = -1
	Stdout   StdStreamType = 0
	Stderr   StdStreamType = 1
	// TypeWorkloadID carries the workload ID, not stream data.
	TypeWorkloadID StdStreamType = 6
)

type RemoveWorkloadMessage struct {
	WorkloadID string
	Success    bool
	Hook       []*bytes.Buffer
}

type DissociateWorkloadMessage struct {
	WorkloadID string
	Error      error
}

type BuildImageMessage struct {
	ID          string      `json:"id,omitempty"`
	Status      string      `json:"status,omitempty"`
	Progress    string      `json:"progress,omitempty"`
	Error       string      `json:"error,omitempty"`
	Stream      string      `json:"stream,omitempty"`
	ErrorDetail errorDetail `json:"errorDetail,omitzero"`
}

type CopyMessage struct {
	ID        string `json:"id,omitempty"`
	Path      string `json:"path,omitempty"`
	Error     error  `json:"error,omitempty"`
	LinuxFile `json:"-"`
}

type SendMessage struct {
	ID    string `json:"id,omitempty"`
	Path  string `json:"path,omitempty"`
	Error error  `json:"error,omitempty"`
}

type CacheImageMessage struct {
	Image    string
	Success  bool
	Nodename string
	Message  string
}

type RemoveImageMessage struct {
	Image    string
	Success  bool
	Messages []string
}

type Image struct {
	ID   string
	Tags []string
}

type ListImageMessage struct {
	Images   []*Image
	Nodename string
	Error    error
}

type ControlWorkloadMessage struct {
	WorkloadID string
	Error      error
	Hook       []*bytes.Buffer
}

type CreateWorkloadMessage struct {
	EngineParams resourcetypes.Resources
	Resources    resourcetypes.Resources
	Podname      string
	Nodename     string
	WorkloadID   string
	WorkloadName string
	Error        error
	Publish      map[string][]string
	Hook         []*bytes.Buffer
}

type ReplaceWorkloadMessage struct {
	Create *CreateWorkloadMessage
	Remove *RemoveWorkloadMessage
	Error  error
}

// AttachWorkloadMessage carries RunAndWait output.
type AttachWorkloadMessage struct {
	WorkloadID string
	Data       []byte
	StdStreamType
}

type StdStreamMessage struct {
	Data []byte
	StdStreamType
}

type LogStreamMessage struct {
	ID    string
	Error error
	Data  []byte
	StdStreamType
}

// CapacityMessage carries CalculateCapacity output.
type CapacityMessage struct {
	Total          int
	NodeCapacities map[string]int
}

type RawEngineMessage struct {
	ID   string `json:"id,omitempty"`
	Data []byte `json:"data,omitempty"`
}

type errorDetail struct {
	Code    int    `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}
