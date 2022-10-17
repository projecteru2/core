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
func (c *Calcium) ControlWorkload(ctx context.Context, ids []string, t string, force bool) (chan *types.ControlWorkloadMessage, error) {
	logger := log.WithField("Calcium", "ControlWorkload").WithField("ids", ids).WithField("t", t).WithField("force", force)
	ch := make(chan *types.ControlWorkloadMessage)

	_ = c.pool.Invoke(func() {
		defer close(ch)
		wg := &sync.WaitGroup{}
		wg.Add(len(ids))
		defer wg.Wait()
		for _, id := range ids {
			id := id
			_ = c.pool.Invoke(func() {
				defer wg.Done()
				var message []*bytes.Buffer
				err := c.withWorkloadLocked(ctx, id, func(ctx context.Context, workload *types.Workload) error {
					var err error
					switch t {
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
					}
					return types.ErrUnknownControlType
				})
				if err == nil {
					logger.Infof(ctx, "[ControlWorkload] Workload %s %s", id, t)
					logger.Infof(ctx, "%v", "[ControlWorkload] Hook Output:")
					logger.Infof(ctx, "%v", string(utils.MergeHookOutputs(message)))
				}
				logger.Errorf(ctx, err, "")
				ch <- &types.ControlWorkloadMessage{
					WorkloadID: id,
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
