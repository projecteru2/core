package types

import "io"

// RemoveContainerMessage for remove message
type RemoveContainerMessage struct {
	ContainerID string
	Success     bool
	Message     string
}

type errorDetail struct {
	Code    int    `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

// BuildImageMessage for build image message
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

// RemoveImageMessage for remove image message
type RemoveImageMessage struct {
	Image    string
	Success  bool
	Messages []string
}

// CreateContainerMessage for create message
type CreateContainerMessage struct {
	Podname       string
	Nodename      string
	ContainerID   string
	ContainerName string
	Error         error
	Success       bool
	CPU           CPUMap
	Quota         float64
	Memory        int64
	Publish       map[string]string
	Hook          []byte
}

// ReplaceContainerMessage for replace method
type ReplaceContainerMessage struct {
	CreateContainerMessage
	OldContainerID string
	Error          error
}

// RunAndWaitMessage for run and wait
type RunAndWaitMessage struct {
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
	Success     bool
}
