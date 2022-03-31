package types

import (
	"context"
	"time"

	"github.com/projecteru2/core/engine"
	enginetypes "github.com/projecteru2/core/engine/types"

	"github.com/pkg/errors"
)

// StatusMeta indicate contaienr runtime
type StatusMeta struct {
	ID string `json:"id"`

	Networks  map[string]string `json:"networks,omitempty"`
	Running   bool              `json:"running,omitempty"`
	Healthy   bool              `json:"healthy,omitempty"`
	Extension []byte            `json:"extension,omitempty"`

	// These attributes are only used when setting workload status.
	Appname    string `json:"-"`
	Nodename   string `json:"-"`
	Entrypoint string `json:"-"`
}

// LabelMeta bind meta info store in labels
type LabelMeta struct {
	Publish     []string
	HealthCheck *HealthCheck
}

// Workload store workload info
// only relationship with pod and node is stored
// if you wanna get realtime information, use Inspect method
type Workload struct {
	ResourceMeta
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	Podname    string            `json:"podname"`
	Nodename   string            `json:"nodename"`
	Hook       *Hook             `json:"hook"`
	Privileged bool              `json:"privileged"`
	User       string            `json:"user"`
	Env        []string          `json:"env"`
	Image      string            `json:"image"`
	Labels     map[string]string `json:"labels"`
	CreateTime int64             `json:"create_time"`
	StatusMeta *StatusMeta       `json:"-"`
	Engine     engine.API        `json:"-"`
}

// Inspect a workload
func (c *Workload) Inspect(ctx context.Context) (*enginetypes.VirtualizationInfo, error) {
	if c.Engine == nil {
		return nil, errors.WithStack(ErrNilEngine)
	}
	info, err := c.Engine.VirtualizationInspect(ctx, c.ID)
	return info, errors.WithStack(err)
}

// Start a workload
func (c *Workload) Start(ctx context.Context) error {
	if c.Engine == nil {
		return errors.WithStack(ErrNilEngine)
	}
	return errors.WithStack(c.Engine.VirtualizationStart(ctx, c.ID))
}

// Stop a workload
func (c *Workload) Stop(ctx context.Context, force bool) error {
	if c.Engine == nil {
		return errors.WithStack(ErrNilEngine)
	}
	gracefulTimeout := time.Duration(-1) // -1 indicates use engine default timeout
	if force {
		gracefulTimeout = 0 // don't wait, kill -15 && kill -9
	}
	return errors.WithStack(c.Engine.VirtualizationStop(ctx, c.ID, gracefulTimeout))
}

// Remove a workload
func (c *Workload) Remove(ctx context.Context, force bool) (err error) {
	if c.Engine == nil {
		return errors.WithStack(ErrNilEngine)
	}
	if err = c.Engine.VirtualizationRemove(ctx, c.ID, true, force); errors.Is(err, ErrWorkloadNotExists) {
		err = nil
	}
	return errors.WithStack(err)
}

// WorkloadStatus store deploy status
type WorkloadStatus struct {
	ID       string
	Workload *Workload
	Error    error
	Delete   bool
}
