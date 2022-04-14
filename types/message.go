package types

import (
	"bytes"
)

// RemoveWorkloadMessage for remove message
type RemoveWorkloadMessage struct {
	WorkloadID string
	Success    bool
	Hook       []*bytes.Buffer
}

// DissociateWorkloadMessage for dissociate workload message
type DissociateWorkloadMessage struct {
	WorkloadID string
	Error      error
}

// BuildImageMessage for build image ops message
type BuildImageMessage struct {
	ID          string      `json:"id,omitempty"`
	Status      string      `json:"status,omitempty"`
	Progress    string      `json:"progress,omitempty"`
	Error       string      `json:"error,omitempty"`
	Stream      string      `json:"stream,omitempty"`
	ErrorDetail errorDetail `json:"errorDetail,omitempty"`
}

// CopyMessage for copy message
type CopyMessage struct {
	ID        string `json:"id,omitempty"`
	Path      string `json:"path,omitempty"`
	Error     error  `json:"error,omitempty"`
	LinuxFile `json:"-"`
}

// SendMessage for send message
type SendMessage struct {
	ID    string `json:"id,omitempty"`
	Path  string `json:"path,omitempty"`
	Error error  `json:"error,omitempty"`
}

// CacheImageMessage for cache image on pod
type CacheImageMessage struct {
	Image    string
	Success  bool
	Nodename string
	Message  string
}

// RemoveImageMessage for remove image message
type RemoveImageMessage struct {
	Image    string
	Success  bool
	Messages []string
}

// Image .
type Image struct {
	ID   string
	Tags []string
}

// ListImageMessage for list image
type ListImageMessage struct {
	Images   []*Image
	Nodename string
	Error    error
}

// ControlWorkloadMessage for workload control message
type ControlWorkloadMessage struct {
	WorkloadID string
	Error      error
	Hook       []*bytes.Buffer
}

// CreateWorkloadMessage for create message
type CreateWorkloadMessage struct {
	EngineArgs   EngineArgs
	ResourceArgs map[string]WorkloadResourceArgs
	Podname      string
	Nodename     string
	WorkloadID   string
	WorkloadName string
	Error        error
	Publish      map[string][]string
	Hook         []*bytes.Buffer
}

// ReplaceWorkloadMessage for replace method
type ReplaceWorkloadMessage struct {
	Create *CreateWorkloadMessage
	Remove *RemoveWorkloadMessage
	Error  error
}

// StdStreamType shows stdout / stderr
type StdStreamType int

const (
	// EruError means this message is carrying some error from eru
	// not from user program
	EruError StdStreamType = -1
	// Stdout means this message is carrying stdout from user program
	Stdout StdStreamType = 0
	// Stderr means this message is carrying stderr from user program
	Stderr StdStreamType = 1
	// TypeWorkloadID means this is the workload id
	TypeWorkloadID StdStreamType = 6
)

// AttachWorkloadMessage for run and wait
type AttachWorkloadMessage struct {
	WorkloadID string
	Data       []byte
	StdStreamType
}

// PullImageMessage for cache image
type PullImageMessage struct {
	BuildImageMessage
}

// ReallocResourceMessage for realloc resource
type ReallocResourceMessage struct {
	WorkloadID string
}

// StdStreamMessage embodies bytes and std type
type StdStreamMessage struct {
	Data []byte
	StdStreamType
}

// LogStreamMessage for log stream
type LogStreamMessage struct {
	ID    string
	Error error
	Data  []byte
	StdStreamType
}

// CapacityMessage for CalculateCapacity API output
type CapacityMessage struct {
	Total          int
	NodeCapacities map[string]int
}

type errorDetail struct {
	Code    int    `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}
