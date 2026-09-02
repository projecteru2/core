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

func (c *Calcium) ControlWorkload(ctx context.Context, IDs []string, typ string, force bool) (chan *types.ControlWorkloadMessage, error) {
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
				err := c.withWorkloadLocked(ctx, ID, false, func(ctx context.Context, workload *types.Workload) error {
					var err error
					switch typ {
					case cluster.WorkloadStop:
						message, err = c.doStopWorkload(ctx, workload, force)
						return err
					case cluster.WorkloadStart:
						message, err = c.doStartWorkload(ctx, workload, force)
						return err
					case cluster.WorkloadRestart:
						message, err = c.doStopWorkload(ctx, workload, force)
						if err != nil {
							return err
						}
						var startHook []*bytes.Buffer
						startHook, err = c.doStartWorkload(ctx, workload, force)
						message = append(message, startHook...)
						return err
					case cluster.WorkloadSuspend:
						message, err = c.doSuspendWorkload(ctx, workload, force)
						return err
					case cluster.WorkloadResume:
						message, err = c.doResumeWorkload(ctx, workload, force)
						return err
					}
					return types.ErrInvaildControlType
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
		message, err = c.doHook(
			ctx,
			workload.ID, workload.User,
			workload.Hook.AfterStart, workload.Env,
			workload.Hook.Force, workload.Privileged,
			force, workload.Engine,
		)
	}
	return message, err
}

func (c *Calcium) doStopWorkload(ctx context.Context, workload *types.Workload, force bool) (message []*bytes.Buffer, err error) {
	if workload.Hook != nil && len(workload.Hook.BeforeStop) > 0 {
		message, err = c.doHook(
			ctx,
			workload.ID, workload.User,
			workload.Hook.BeforeStop, workload.Env,
			workload.Hook.Force, workload.Privileged,
			force, workload.Engine,
		)
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
		message, err = c.doHook(
			ctx,
			workload.ID, workload.User,
			workload.Hook.BeforeSuspend, workload.Env,
			workload.Hook.Force, workload.Privileged,
			force, workload.Engine,
		)
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
		message, err = c.doHook(
			ctx,
			workload.ID, workload.User,
			workload.Hook.AfterResume, workload.Env,
			workload.Hook.Force, workload.Privileged,
			force, workload.Engine,
		)
	}
	return message, err
}
