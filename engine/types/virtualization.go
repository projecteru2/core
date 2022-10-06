package types

// VirtualizationResource define resources
type VirtualizationResource struct {
	EngineArgs    map[string]interface{}      `json:"-" mapstructure:"-"`
	CPU           map[string]int64            `json:"cpu_map" mapstructure:"cpu_map"` // for cpu binding
	Quota         float64                     `json:"cpu" mapstructure:"cpu"`         // for cpu quota
	Memory        int64                       `json:"memory" mapstructure:"memory"`   // for memory binding
	Storage       int64                       `json:"storage" mapstructure:"storage"`
	NUMANode      string                      `json:"numa_node" mapstructure:"numa_node"` // numa node
	Volumes       []string                    `json:"volumes" mapstructure:"volumes"`
	VolumePlan    map[string]map[string]int64 `json:"volume_plan" mapstructure:"volume_plan"`       // literal VolumePlan
	VolumeChanged bool                        `json:"volume_changed" mapstructure:"volume_changed"` // indicate whether new volumes contained in realloc request
	IOPSOptions   map[string]string           `json:"iops_options" mapstructure:"iops_options"`     // format: {device_name: "read-iops:write-iops:read-bps:write-bps"}
	Remap         bool                        `json:"remap" mapstructure:"remap"`
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

	Debug   bool
	Restart string

	Networks map[string]string

	LogType   string
	LogConfig map[string]string

	RawArgs []byte
	Lambda  bool

	AncestorWorkloadID string
}

// VirtualizationCreated use for store name and ID
type VirtualizationCreated struct {
	ID     string
	Name   string
	Labels map[string]string
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

// VirtualizationRemapOptions is passed to engine
type VirtualizationRemapOptions struct {
	CPUAvailable      map[string]int64
	CPUInit           map[string]int64 // engine can be aware of oversell
	CPUShareBase      int64
	WorkloadResources map[string]VirtualizationResource
}

// VirtualizationRemapMessage returns from engine
type VirtualizationRemapMessage struct {
	ID    string
	Error error
}
