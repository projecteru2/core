package types

// VirtualizationResource define resources
type VirtualizationResource struct {
	CPU           map[string]int64 // for cpu binding
	Quota         float64          // for cpu quota
	Memory        int64            // for memory binding
	Storage       int64
	NUMANode      string // numa node
	Volumes       []string
	VolumePlan    map[string]map[string]int64 // literal VolumePlan
	VolumeChanged bool                        // indicate whether new volumes contained in realloc request
}

// VirtualizationCreateOptions use for create virtualization target
type VirtualizationCreateOptions struct {
	VirtualizationResource
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

	Debug bool

	RestartPolicy string

	Networks map[string]string

	Volumes []string

	LogType   string
	LogConfig map[string]string

	RawArgs []byte
	Lambda  bool

	AncestorWorkloadID string
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
	// TODO other information like cpu memory
}

// VirtualizationWaitResult store exit result
type VirtualizationWaitResult struct {
	Message string
	Code    int64
}
