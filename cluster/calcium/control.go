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

// ControlWorkload control workloads status
func (c *Calcium) ControlWorkload(ctx context.Context, IDs []string, typ string, force bool) (chan *types.ControlWorkloadMessage, error) {
	logger := log.WithFunc("calcium.ControlWorkload").WithField("IDs", IDs).WithField("typ", typ).WithField("force", force)
	ch := make(chan *types.ControlWorkloadMessage)

	_ = c.pool.Invoke(func() {
		defer close(ch)
		wg := &sync.WaitGroup{}
		wg.Add(len(IDs))
		defer wg.Wait()
		for _, ID := range IDs {
			ID := ID
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
						startHook, err := c.doStartWorkload(ctx, workload, force)
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
					logger.Infof(ctx, "Workload %s %s", ID, typ)
					logger.Infof(ctx, "%+v", "Hook Output:")
					logger.Infof(ctx, "%+v", string(utils.MergeHookOutputs(message)))
				}
				logger.Error(ctx, err)
				ch <- &types.ControlWorkloadMessage{
					WorkloadID: ID,
					Error:      err,
					Hook:       message,
				}
			})
		}
	})

	return ch, nil
}

func (c *Calcium) doStartWorkload(ctx context.Context, workload *types.Workload, force bool) (message []*bytes.Buffer, err error) {
	if err = workload.Start(ctx); err != nil {
		return message, err
	}
	// TODO healthcheck first
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

	// 这里 block 的问题很严重，按照目前的配置是 5 分钟一级的 block
	// 一个简单的处理方法是相信 ctx 不相信 engine 自身的处理
	// 另外我怀疑 engine 自己的 timeout 实现是完全的等 timeout 而非结束了就退出
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
