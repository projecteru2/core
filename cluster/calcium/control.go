package calcium

import (
	"bytes"
	"context"
	"sync"

	"github.com/projecteru2/core/cluster"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

type controlHandler func(context.Context, *types.Workload, bool) ([]*bytes.Buffer, error)

func (c *Calcium) ControlWorkload(ctx context.Context, IDs []string, typ string, force bool) (chan *types.ControlWorkloadMessage, error) {
	handlers := map[string]controlHandler{
		cluster.WorkloadStop:    c.doStopWorkload,
		cluster.WorkloadStart:   c.doStartWorkload,
		cluster.WorkloadRestart: c.doRestartWorkload,
		cluster.WorkloadSuspend: c.doSuspendWorkload,
		cluster.WorkloadResume:  c.doResumeWorkload,
	}
	handle, ok := handlers[typ]
	if !ok {
		return nil, types.ErrInvaildControlType
	}

	logger := log.WithFunc("calcium.ControlWorkload").WithField("IDs", IDs).WithField("typ", typ).WithField("force", force)
	ch := make(chan *types.ControlWorkloadMessage)

	utils.SentryGo(func() {
		defer close(ch)
		wg := &sync.WaitGroup{}
		wg.Add(len(IDs))
		defer wg.Wait()
		for _, ID := range IDs {
			_ = c.pool.Invoke(func() {
				defer wg.Done()
				var message []*bytes.Buffer
				err := c.withWorkloadLocked(ctx, ID, false, func(ctx context.Context, workload *types.Workload) (err error) {
					message, err = handle(ctx, workload, force)
					return err
				})
				if err == nil {
					logger.Infof(ctx, "workload %s %s", ID, typ)
					logger.Info(ctx, string(utils.MergeHookOutputs(message)))
				} else {
					logger.Error(ctx, err)
				}
				_ = send(ctx, ch, &types.ControlWorkloadMessage{
					WorkloadID: ID,
					Error:      err,
					Hook:       message,
				})
			})
		}
	})

	return ch, nil
}

func (c *Calcium) doStartWorkload(ctx context.Context, workload *types.Workload, force bool) (message []*bytes.Buffer, err error) {
	if err = workload.Start(ctx); err != nil {
		return message, err
	}
	if workload.Hook != nil && len(workload.Hook.AfterStart) > 0 {
		message, err = c.doHook(ctx, workload, workload.Hook.AfterStart, force)
	}
	return message, err
}

func (c *Calcium) doRestartWorkload(ctx context.Context, workload *types.Workload, force bool) ([]*bytes.Buffer, error) {
	message, err := c.doStopWorkload(ctx, workload, force)
	if err != nil {
		return message, err
	}
	startHook, err := c.doStartWorkload(ctx, workload, force)
	return append(message, startHook...), err
}

func (c *Calcium) doStopWorkload(ctx context.Context, workload *types.Workload, force bool) (message []*bytes.Buffer, err error) {
	if workload.Hook != nil && len(workload.Hook.BeforeStop) > 0 {
		message, err = c.doHook(ctx, workload, workload.Hook.BeforeStop, force)
		if err != nil {
			return message, err
		}
	}

	if err = workload.Stop(ctx, force); err != nil {
		message = append(message, bytes.NewBufferString(err.Error()))
	}
	return message, err
}

func (c *Calcium) doSuspendWorkload(ctx context.Context, workload *types.Workload, force bool) (message []*bytes.Buffer, err error) {
	if workload.Hook != nil && len(workload.Hook.BeforeSuspend) > 0 {
		message, err = c.doHook(ctx, workload, workload.Hook.BeforeSuspend, force)
		if err != nil {
			return message, err
		}
	}

	if err = workload.Suspend(ctx); err != nil {
		message = append(message, bytes.NewBufferString(err.Error()))
	}
	return message, err
}

func (c *Calcium) doResumeWorkload(ctx context.Context, workload *types.Workload, force bool) (message []*bytes.Buffer, err error) {
	if err = workload.Resume(ctx); err != nil {
		return message, err
	}
	if workload.Hook != nil && len(workload.Hook.AfterResume) > 0 {
		message, err = c.doHook(ctx, workload, workload.Hook.AfterResume, force)
	}
	return message, err
}
