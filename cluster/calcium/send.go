package calcium

import (
	"bytes"
	"context"
	"io"
	"sync"

	"github.com/projecteru2/core/engine"
	"github.com/projecteru2/core/types"
	log "github.com/sirupsen/logrus"
)

// Send send files to workload
func (c *Calcium) Send(ctx context.Context, opts *types.SendOptions) (chan *types.SendMessage, error) {
	ch := make(chan *types.SendMessage)
	go func() {
		defer close(ch)
		wg := &sync.WaitGroup{}
		for dst, content := range opts.Data {
			log.Infof("[Send] Send files to %s", dst)
			wg.Add(1)
			go func(dst string, content []byte) {
				defer wg.Done()
				for _, ID := range opts.IDs {
					workload, err := c.GetWorkload(ctx, ID)
					if err != nil {
						ch <- &types.SendMessage{ID: ID, Path: dst, Error: err}
						continue
					}
					if err := c.doSendFileToWorkload(ctx, workload.Engine, workload.ID, dst, bytes.NewBuffer(content), true, true); err != nil {
						ch <- &types.SendMessage{ID: ID, Path: dst, Error: err}
						continue
					}
					ch <- &types.SendMessage{ID: ID, Path: dst}
				}
			}(dst, content)
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
