package types

import "io"

type RemoveContainerMessage struct {
	ContainerID string
	Success     bool
	Message     string
}

type errorDetail struct {
	Code    int    `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

type BuildImageMessage struct {
	ID          string      `json:"id,omitempty"`
	Status      string      `json:"status,omitempty"`
	Progress    string      `json:"progress,omitempty"`
	Error       string      `json:"error,omitempty"`
	Stream      string      `json:"stream,omitempty"`
	ErrorDetail errorDetail `json:"errorDetail,omitempty"`
}

type CopyMessage struct {
	ID     string        `json:"id,omitempty"`
	Status string        `json:"status,omitempty"`
	Name   string        `json:"name,omitempty"`
	Path   string        `json:"path,omitempty"`
	Error  error         `json:"error,omitempty"`
	Data   io.ReadCloser `json:"-"`
}

type RemoveImageMessage struct {
	Image    string
	Success  bool
	Messages []string
}

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

type RunAndWaitMessage struct {
	ContainerID string
	Data        []byte
}

type PullImageMessage struct {
	BuildImageMessage
}

type ReallocResourceMessage struct {
	ContainerID string
	Success     bool
}
