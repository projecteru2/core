package calcium

import (
	"bytes"
	"context"
	"sync"

	"github.com/projecteru2/core/engine"
	"github.com/projecteru2/core/types"
	log "github.com/sirupsen/logrus"
)

// Send send files to container
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
					container, err := c.GetContainer(ctx, ID)
					if err != nil {
						ch <- &types.SendMessage{ID: ID, Path: dst, Error: err}
						continue
					}
					if err := c.doSendFileToContainer(ctx, container.Engine, container.ID, dst, content, true, true); err != nil {
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

func (c *Calcium) doSendFileToContainer(ctx context.Context, engine engine.API, ID, dst string, content []byte, AllowOverwriteDirWithFile bool, CopyUIDGID bool) error {
	log.Infof("[doSendFileToContainer] Send file to %s:%s", ID, dst)
	log.Debugf("[doSendFileToContainer] remote path %s", dst)
	return engine.VirtualizationCopyTo(ctx, ID, dst, bytes.NewBuffer(content), AllowOverwriteDirWithFile, CopyUIDGID)
}
