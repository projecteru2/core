package types

import (
	"bytes"
	"io"
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
	ID    string        `json:"id,omitempty"`
	Name  string        `json:"name,omitempty"`
	Path  string        `json:"path,omitempty"`
	Error error         `json:"error,omitempty"`
	Data  io.ReadCloser `json:"-"`
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

// ControlWorkloadMessage for workload control message
type ControlWorkloadMessage struct {
	WorkloadID string
	Error      error
	Hook       []*bytes.Buffer
}

// CreateWorkloadMessage for create message
type CreateWorkloadMessage struct {
	ResourceMeta
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

// AttachWorkloadMessage for run and wait
type AttachWorkloadMessage struct {
	WorkloadID string
	Data       []byte
}

// PullImageMessage for cache image
type PullImageMessage struct {
	BuildImageMessage
}

// ReallocResourceMessage for realloc resource
type ReallocResourceMessage struct {
	WorkloadID string
}

// LogStreamMessage for log stream
type LogStreamMessage struct {
	ID    string
	Error error
	Data  []byte
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
