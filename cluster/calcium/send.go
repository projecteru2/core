package calcium

import (
	"context"
	"os"
	"path/filepath"

	"github.com/projecteru2/core/engine"
	"github.com/projecteru2/core/types"
	log "github.com/sirupsen/logrus"
)

// Send send files to container
func (c *Calcium) Send(ctx context.Context, opts *types.SendOptions) (chan *types.SendMessage, error) {
	ch := make(chan *types.SendMessage)
	go func() {
		defer close(ch)
		// TODO use wait group, do it concurrently
		for dst, src := range opts.Data {
			log.Infof("[Send] Send files to containers' %s", dst)
			for _, ID := range opts.IDs {
				container, err := c.GetContainer(ctx, ID)
				if err != nil {
					ch <- &types.SendMessage{ID: ID, Path: dst, Error: err}
					continue
				}
				if err := c.doSendFileToContainer(ctx, container.Engine, container.ID, dst, src, true, true); err != nil {
					ch <- &types.SendMessage{ID: ID, Path: dst, Error: err}
					continue
				}
				ch <- &types.SendMessage{ID: ID, Path: dst}
			}
		}
	}()
	return ch, nil
}

func (c *Calcium) doSendFileToContainer(ctx context.Context, engine engine.API, ID, dst, src string, AllowOverwriteDirWithFile bool, CopyUIDGID bool) error {
	path := filepath.Dir(dst)
	filename := filepath.Base(dst)
	log.Infof("[doSendFileToContainer] Send file %s to dir %s", filename, path)
	log.Debugf("[doSendFileToContainer] Local file %s, remote path %s", src, dst)
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()
	return engine.VirtualizationCopyTo(ctx, ID, path, f, AllowOverwriteDirWithFile, CopyUIDGID)
}
