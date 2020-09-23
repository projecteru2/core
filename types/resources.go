package types

type CPUMemResourceRequest struct {
	CPUQuota float64
	CPUBind  bool
	Memory   int64
}

type StorageResourceRequest struct {
	Quota int64
}

type VolumeResourceRequest struct {
	VolumeBindings
}
