package types

import (
	"context"
	"time"

	engine "github.com/projecteru2/core/engine"
	enginetypes "github.com/projecteru2/core/engine/types"
)

// Container store container info
// only relationship with pod and node is stored
// if you wanna get realtime information, use Inspect method
type Container struct {
	ID         string     `json:"id"`
	Podname    string     `json:"podname"`
	Nodename   string     `json:"nodename"`
	Name       string     `json:"name"`
	CPU        CPUMap     `json:"cpu"`
	Quota      float64    `json:"quota"`
	Memory     int64      `json:"memory"`
	Storage    int64      `json:"storage"`
	Hook       *Hook      `json:"hook"`
	Privileged bool       `json:"privileged"`
	SoftLimit  bool       `json:"softlimit"`
	StatusData []byte     `json:"-"`
	Engine     engine.API `json:"-"`
}

// Inspect a container
func (c *Container) Inspect(ctx context.Context) (*enginetypes.VirtualizationInfo, error) {
	if c.Engine == nil {
		return nil, ErrNilEngine
	}
	// TODO remove it later like start stop and remove
	inspectCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	return c.Engine.VirtualizationInspect(inspectCtx, c.ID)
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

// DeployStatus store deploy status
type DeployStatus struct {
	Data       string
	Error      error
	Action     string
	Appname    string
	Entrypoint string
	Nodename   string
	ID         string
}

// EruMeta bind meta info store in labels
type EruMeta struct {
	Publish     []string
	HealthCheck *HealthCheck
}
