package calcium

import (
	"bytes"
	"context"
	"sync"

	"github.com/projecteru2/core/store"
	"github.com/projecteru2/core/utils"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
)

// RemoveWorkload remove workloads
// returns a channel that contains removing responses
func (c *Calcium) RemoveWorkload(ctx context.Context, IDs []string, force bool, step int) (chan *types.RemoveWorkloadMessage, error) {
	ch := make(chan *types.RemoveWorkloadMessage)
	if step < 1 {
		step = 1
	}

	go func() {
		defer close(ch)
		wg := sync.WaitGroup{}
		defer wg.Wait()
		for i, ID := range IDs {
			wg.Add(1)
			go func(ID string) {
				defer wg.Done()
				ret := &types.RemoveWorkloadMessage{WorkloadID: ID, Success: false, Hook: []*bytes.Buffer{}}
				if err := c.withWorkloadLocked(ctx, ID, func(ctx context.Context, workload *types.Workload) error {
					return c.withNodeLocked(ctx, workload.Nodename, func(ctx context.Context, node *types.Node) (err error) {
						return utils.Txn(
							ctx,
							// if
							func(ctx context.Context) error {
								return c.doRemoveWorkload(ctx, workload, force)
							},
							// then
							func(ctx context.Context) error {
								log.Infof("[RemoveWorkload] Workload %s removed", workload.ID)
								return c.store.UpdateNodeResource(ctx, node, &workload.ResourceMeta, store.ActionIncr)
							},
							// rollback
							nil,
							c.config.GlobalTimeout,
						)
					})
				}); err != nil {
					log.Errorf("[RemoveWorkload] Remove workload %s failed, err: %v", ID, err)
					ret.Hook = append(ret.Hook, bytes.NewBufferString(err.Error()))
				} else {
					ret.Success = true
				}
				ch <- ret
			}(ID)
			if (i+1)%step == 0 {
				log.Info("[RemoveWorkload] Wait for previous tasks done")
				wg.Wait()
			}
		}
	}()
	return ch, nil
}

func (c *Calcium) doRemoveWorkload(ctx context.Context, workload *types.Workload, force bool) error {
	return utils.Txn(
		ctx,
		// if
		func(ctx context.Context) error {
			return workload.Remove(ctx, force)
		},
		// then
		func(ctx context.Context) error {
			return c.store.RemoveWorkload(ctx, workload)
		},
		// rollback
		nil,
		c.config.GlobalTimeout,
	)

}

// 同步地删除容器, 在某些需要等待的场合异常有用!
func (c *Calcium) doRemoveWorkloadSync(ctx context.Context, IDs []string) error {
	ch, err := c.RemoveWorkload(ctx, IDs, true, 1)
	if err != nil {
		return err
	}

	for m := range ch {
		log.Debugf("[doRemoveWorkloadSync] Removed %s", m.WorkloadID)
	}
	return nil
}
