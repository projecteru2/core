package types

// EngineParams .
type EngineParams struct {
	CPU      float64 `json:"cpu" mapstructure:"cpu"`
	CPUMap   CPUMap  `json:"cpu_map" mapstructure:"cpu_map"`
	NUMANode string  `json:"numa_node" mapstructure:"numa_node"`
	Memory   int64   `json:"memory" mapstructure:"memory"`
	Remap    bool    `json:"remap" mapstructure:"remap"`
}
