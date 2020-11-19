package types

import (
	"bytes"
	"io"
)

// RemoveContainerMessage for remove message
type RemoveContainerMessage struct {
	ContainerID string
	Success     bool
	Hook        []*bytes.Buffer
}

// DissociateContainerMessage for dissociate container message
type DissociateContainerMessage struct {
	ContainerID string
	Error       error
}

type errorDetail struct {
	Code    int    `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
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
	ID     string        `json:"id,omitempty"`
	Status string        `json:"status,omitempty"`
	Name   string        `json:"name,omitempty"`
	Path   string        `json:"path,omitempty"`
	Error  error         `json:"error,omitempty"`
	Data   io.ReadCloser `json:"-"`
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

// ControlContainerMessage for container control message
type ControlContainerMessage struct {
	ContainerID string
	Error       error
	Hook        []*bytes.Buffer
}

// CreateContainerMessage for create message
type CreateContainerMessage struct {
	Podname       string
	Nodename      string
	ContainerID   string
	ContainerName string
	Error         error
	CPU           CPUMap
	Quota         float64
	Memory        int64
	VolumePlan    VolumePlan
	Storage       int64
	Publish       map[string][]string
	Hook          []*bytes.Buffer
}

// ReplaceContainerMessage for replace method
type ReplaceContainerMessage struct {
	Create *CreateContainerMessage
	Remove *RemoveContainerMessage
	Error  error
}

// AttachContainerMessage for run and wait
type AttachContainerMessage struct {
	ContainerID string
	Data        []byte
}

// PullImageMessage for cache image
type PullImageMessage struct {
	BuildImageMessage
}

// ReallocResourceMessage for realloc resource
type ReallocResourceMessage struct {
	ContainerID string
	Error       error
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
	NodeCapacities map[string]*CapacityInfo
}

// CapacityInfo for CapacityMessage
type CapacityInfo struct {
	Nodename        string
	Capacity        int
	Exist           int
	CapacityCPUMem  int
	CapacityVolume  int
	CapacityStorage int
}
