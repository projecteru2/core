package types

// EngineParams .
type EngineParams struct {
	CPU      float64 `json:"cpu"`
	CPUMap   CPUMap  `json:"cpu_map"`
	NUMANode string  `json:"numa_node"`
	Memory   int64   `json:"memory"`
	Remap    bool    `json:"remap"`
}
