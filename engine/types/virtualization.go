package types

// VirtualizationResource define resources
type VirtualizationResource struct {
	CPU       map[string]int // for cpu binding
	Quota     float64        // for cpu quota
	Memory    int64          // for memory binding
	SoftLimit bool           // soft limit or not
}

// VirtualizationUlimits define hard and soft limit
type VirtualizationUlimits struct {
	Soft int64
	Hard int64
}

// VirtualizationCreateOptions use for create virtualization target
type VirtualizationCreateOptions struct {
	VirtualizationResource
	Seq        int // for count
	Name       string
	User       string
	Image      string
	WorkingDir string
	Stdin      bool
	Privileged bool
	Cmd        []string
	Env        []string
	DNS        []string
	Hosts      []string
	Publish    []string
	Sysctl     map[string]string
	Labels     map[string]string

	CapAdd  []string
	Ulimits map[string]*VirtualizationUlimits

	RestartPolicy     string
	RestartRetryCount int

	Network         string
	Networks        map[string]string
	NetworkDisabled bool

	Binds   []string
	Volumes map[string]struct{}

	LogType   string
	LogConfig map[string]string
}

// VirtualizationCreated use for store name and ID
type VirtualizationCreated struct {
	ID   string
	Name string
}

// VirtualizationInfo store virtualization info
type VirtualizationInfo struct {
	ID       string
	User     string
	Image    string
	Running  bool
	Env      []string
	Labels   map[string]string
	Networks map[string]string
}

// VirtualizationWaitResult store exit result
type VirtualizationWaitResult struct {
	Message string
	Code    int64
}
