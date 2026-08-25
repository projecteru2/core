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

func (w *Workload) Inspect(ctx context.Context) (*enginetypes.VirtualizationInfo, error) {
	return w.Engine.VirtualizationInspect(ctx, w.ID)
}

func (w *Workload) Start(ctx context.Context) error {
	return w.Engine.VirtualizationStart(ctx, w.ID)
}

func (w *Workload) Stop(ctx context.Context, force bool) error {
	gracefulTimeout := time.Duration(-1) // -1 means engine default timeout
	if force {
		gracefulTimeout = 0 // 0 means SIGTERM then SIGKILL, no wait
	}
	return w.Engine.VirtualizationStop(ctx, w.ID, gracefulTimeout)
}

func (w *Workload) Suspend(ctx context.Context) error {
	return w.Engine.VirtualizationSuspend(ctx, w.ID)
}

func (w *Workload) Resume(ctx context.Context) error {
	return w.Engine.VirtualizationResume(ctx, w.ID)
}

func (w *Workload) Remove(ctx context.Context, force bool) (err error) {
	if err = w.Engine.VirtualizationRemove(ctx, w.ID, true, force); errors.Is(err, ErrWorkloadNotExists) {
		err = nil
	}
	return err
}

func (w *Workload) RawEngine(ctx context.Context, opts *RawEngineOptions) (ans *RawEngineMessage, err error) {
	eOpts := &enginetypes.RawEngineOptions{
		ID:     opts.ID,
		Op:     opts.Op,
		Params: opts.Params,
	}
	eResp, err := w.Engine.RawEngine(ctx, eOpts)
	if err != nil {
		return nil, err
	}
	ans = &RawEngineMessage{
		ID:   eResp.ID,
		Data: eResp.Data,
	}
	return ans, nil
}

type WorkloadStatus struct {
	ID       string
	Workload *Workload
	Error    error
	Delete   bool
}
