package types

import (
	"context"
	"time"

	"github.com/cockroachdb/errors"

	"github.com/projecteru2/core/engine"
	enginetypes "github.com/projecteru2/core/engine/types"
	resourcetypes "github.com/projecteru2/core/resource/types"
)

type StatusMeta struct {
	ID string `json:"id"`

	Networks  map[string]string `json:"networks,omitempty"`
	Running   bool              `json:"running,omitempty"`
	Healthy   bool              `json:"healthy,omitempty"`
	Extension []byte            `json:"extension,omitempty"`

	// set only when writing workload status
	Appname    string `json:"-"`
	Nodename   string `json:"-"`
	Entrypoint string `json:"-"`
}

type LabelMeta struct {
	Publish     []string
	HealthCheck *HealthCheck
}

// Workload is the stored pod/node relation; use Inspect for live state.
type Workload struct {
	Resources    resourcetypes.Resources `json:"resources"`
	EngineParams resourcetypes.Resources `json:"engine_params"`
	ID           string                  `json:"id"`
	Name         string                  `json:"name"`
	Podname      string                  `json:"podname"`
	Nodename     string                  `json:"nodename"`
	Hook         *Hook                   `json:"hook"`
	Privileged   bool                    `json:"privileged"`
	User         string                  `json:"user"`
	Env          []string                `json:"env"`
	Image        string                  `json:"image"`
	Labels       map[string]string       `json:"labels"`
	CreateTime   int64                   `json:"create_time"`
	StatusMeta   *StatusMeta             `json:"-"`
	Engine       engine.API              `json:"-"`
}

func (c *Workload) Inspect(ctx context.Context) (*enginetypes.VirtualizationInfo, error) {
	if c.Engine == nil {
		return nil, ErrNilEngine
	}
	info, err := c.Engine.VirtualizationInspect(ctx, c.ID)
	return info, err
}

func (c *Workload) Start(ctx context.Context) error {
	if c.Engine == nil {
		return ErrNilEngine
	}
	return c.Engine.VirtualizationStart(ctx, c.ID)
}

func (c *Workload) Stop(ctx context.Context, force bool) error {
	if c.Engine == nil {
		return ErrNilEngine
	}
	gracefulTimeout := time.Duration(-1) // -1 means engine default timeout
	if force {
		gracefulTimeout = 0 // 0 means SIGTERM then SIGKILL, no wait
	}
	return c.Engine.VirtualizationStop(ctx, c.ID, gracefulTimeout)
}

func (c *Workload) Suspend(ctx context.Context) error {
	if c.Engine == nil {
		return ErrNilEngine
	}
	return c.Engine.VirtualizationSuspend(ctx, c.ID)
}

func (c *Workload) Resume(ctx context.Context) error {
	if c.Engine == nil {
		return ErrNilEngine
	}
	return c.Engine.VirtualizationResume(ctx, c.ID)
}

func (c *Workload) Remove(ctx context.Context, force bool) (err error) {
	if c.Engine == nil {
		return ErrNilEngine
	}
	if err = c.Engine.VirtualizationRemove(ctx, c.ID, true, force); errors.Is(err, ErrWorkloadNotExists) {
		err = nil
	}
	return err
}

func (c *Workload) RawEngine(ctx context.Context, opts *RawEngineOptions) (ans *RawEngineMessage, err error) {
	if c.Engine == nil {
		return nil, ErrNilEngine
	}
	eOpts := &enginetypes.RawEngineOptions{
		ID:     opts.ID,
		Op:     opts.Op,
		Params: opts.Params,
	}
	eResp, err := c.Engine.RawEngine(ctx, eOpts)
	if err != nil {
		return ans, err
	}
	ans = &RawEngineMessage{
		ID:   eResp.ID,
		Data: eResp.Data,
	}
	return ans, err
}

type WorkloadStatus struct {
	ID       string
	Workload *Workload
	Error    error
	Delete   bool
}
