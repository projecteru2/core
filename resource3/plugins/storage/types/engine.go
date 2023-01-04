package types

// EngineParams .
type EngineParams struct {
	Volumes       []string          `json:"volumes"`
	VolumeChanged bool              `json:"volume_changed"` // indicates whether the realloc request includes new volumes
	Storage       int64             `json:"storage"`
	IOPSOptions   map[string]string `json:"iops_options"`
}
