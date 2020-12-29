package calcium

import (
	"bytes"
	"context"
	"io"
	"sync"

	"github.com/projecteru2/core/engine"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
)

// Send send files to workload
func (c *Calcium) Send(ctx context.Context, opts *types.SendOptions) (chan *types.SendMessage, error) {
	if err := opts.Validate(); err != nil {
		return nil, err
	}
	ch := make(chan *types.SendMessage)
	go func() {
		defer close(ch)
		wg := &sync.WaitGroup{}

		for _, ID := range opts.IDs {
			log.Infof("[Send] Send files to %s", ID)
			wg.Add(1)
			go func(ID string) {
				defer wg.Done()
				if err := c.withWorkloadLocked(ctx, ID, func(ctx context.Context, workload *types.Workload) error {
					for dst, content := range opts.Data {
						err := c.doSendFileToWorkload(ctx, workload.Engine, workload.ID, dst, bytes.NewBuffer(content), true, true)
						ch <- &types.SendMessage{ID: ID, Path: dst, Error: err}
					}
					return nil
				}); err != nil {
					ch <- &types.SendMessage{ID: ID, Error: err}
				}
			}(ID)
		}
		wg.Wait()
	}()
	return ch, nil
}

func (c *Calcium) doSendFileToWorkload(ctx context.Context, engine engine.API, ID, dst string, content io.Reader, AllowOverwriteDirWithFile bool, CopyUIDGID bool) error {
	log.Infof("[doSendFileToWorkload] Send file to %s:%s", ID, dst)
	log.Debugf("[doSendFileToWorkload] remote path %s", dst)
	return engine.VirtualizationCopyTo(ctx, ID, dst, content, AllowOverwriteDirWithFile, CopyUIDGID)
}
