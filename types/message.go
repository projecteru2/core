package types

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
	Status      string      `json:"status,omitempty"`
	Progress    string      `json:"progress,omitempty"`
	Error       string      `json:"error,omitempty"`
	Stream      string      `json:"stream,omitempty"`
	ErrorDetail errorDetail `json:"errorDetail,omitempty"`
}

type BackupMessage struct {
	Status string `json:"status,omitempty"`
	Size   int64  `json:"size,omitempty"`
	Error  string `json:"error,omitempty"`
	Path   string `json:"path,omitempty"`
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
	Error         string
	Success       bool
	CPU           CPUMap
	Memory        int64
	Publish       map[string]string
	HookOutput    []byte
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
