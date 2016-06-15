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

type RemoveImageMessage struct {
	Image    string
	Success  bool
	Messages []string
}
