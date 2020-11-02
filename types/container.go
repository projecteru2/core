package types

import (
	"context"
	"encoding/json"

	engine "github.com/projecteru2/core/engine"
	enginetypes "github.com/projecteru2/core/engine/types"
)

// StatusMeta indicate contaienr runtime
type StatusMeta struct {
	ID string `json:"id"`

	Networks  map[string]string `json:"networks,omitempty"`
	Running   bool              `json:"running,omitempty"`
	Healthy   bool              `json:"healthy,omitempty"`
	Extension []byte            `json:"extension,omitempty"`
}

// LabelMeta bind meta info store in labels
type LabelMeta struct {
	Publish     []string
	HealthCheck *HealthCheck
}

// Container store container info
// only relationship with pod and node is stored
// if you wanna get realtime information, use Inspect method
type Container struct {
	ID                   string            `json:"id"`
	Name                 string            `json:"name"`
	Podname              string            `json:"podname"`
	Nodename             string            `json:"nodename"`
	CPURequest           CPUMap            `json:"cpu"`
	CPULimit             CPUMap            `json:"cpu_limit"`
	QuotaRequest         float64           `json:"quota"`
	QuotaLimit           float64           `json:"quota_limit"`
	MemoryRequest        int64             `json:"memory"`
	MemoryLimit          int64             `json:"memory_limit"`
	StorageRequest       int64             `json:"storage"`
	StorageLimit         int64             `json:"storage_limit"`
	Hook                 *Hook             `json:"hook"`
	Privileged           bool              `json:"privileged"`
	SoftLimit            bool              `json:"softlimit"`
	User                 string            `json:"user"`
	Env                  []string          `json:"env"`
	Image                string            `json:"image"`
	VolumeRequest        VolumeBindings    `json:"volumes"`
	VolumePlanRequest    VolumePlan        `json:"volume_plan"`
	VolumeLimit          VolumeBindings    `json:"volumes_limit"`
	VolumePlanLimit      VolumePlan        `json:"volume_plan_limit"`
	Labels               map[string]string `json:"labels"`
	StatusMeta           *StatusMeta       `json:"-"`
	Engine               engine.API        `json:"-"`
	ResourceSubdivisible bool              `json:"resource_subdivisible"` // to tell apart from existing container metas
}

type container Container

// UnmarshalJSON makes compatible for old resource fields
func (c *Container) UnmarshalJSON(b []byte) error {
	cc := &container{}
	if err := json.Unmarshal(b, cc); err != nil {
		return err
	}

	*c = Container(*cc)

	// existing container meta
	if !c.ResourceSubdivisible {
		c.QuotaLimit = c.QuotaRequest
		c.CPULimit = c.CPURequest
		c.StorageLimit = c.StorageRequest
		c.MemoryLimit = c.MemoryRequest
		c.VolumeLimit = c.VolumeRequest
		c.VolumePlanLimit = c.VolumePlanRequest
	}
	return nil
}

// Inspect a container
func (c *Container) Inspect(ctx context.Context) (*enginetypes.VirtualizationInfo, error) {
	if c.Engine == nil {
		return nil, ErrNilEngine
	}
	return c.Engine.VirtualizationInspect(ctx, c.ID)
}

// Start a container
func (c *Container) Start(ctx context.Context) error {
	if c.Engine == nil {
		return ErrNilEngine
	}
	return c.Engine.VirtualizationStart(ctx, c.ID)
}

// Stop a container
func (c *Container) Stop(ctx context.Context) error {
	if c.Engine == nil {
		return ErrNilEngine
	}
	return c.Engine.VirtualizationStop(ctx, c.ID)
}

// Remove a container
func (c *Container) Remove(ctx context.Context, force bool) error {
	if c.Engine == nil {
		return ErrNilEngine
	}
	return c.Engine.VirtualizationRemove(ctx, c.ID, true, force)
}

// ContainerStatus store deploy status
type ContainerStatus struct {
	ID        string
	Container *Container
	Error     error
	Delete    bool
}
