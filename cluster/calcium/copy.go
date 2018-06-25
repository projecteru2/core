package calcium

import (
	"context"
	"io"
	"sync"

	"github.com/projecteru2/core/types"
	log "github.com/sirupsen/logrus"
)

const (
	COPY_FAILED = "failed"
	COPY_OK     = "ok"
)

//Copy uses docker cp to copy specified things and send to remote
func (c *calcium) Copy(ctx context.Context, opts *types.CopyOptions) (chan *types.CopyMessage, error) {
	ch := make(chan *types.CopyMessage)
	go func() {
		defer close(ch)
		wg := sync.WaitGroup{}
		log.Debugf("[Copy] Copy %d containers files", len(opts.Targets))
		// container one by one
		for cid, paths := range opts.Targets {
			wg.Add(1)
			go func(cid string, paths []string) {
				defer wg.Done()
				container, err := c.GetContainer(cid)
				if err != nil {
					log.Errorf("[Copy] Error when get container %s", cid)
					ch <- makeCopyMessage(cid, COPY_FAILED, "", "", err, nil)
					return
				}

				node, err := c.GetNode(container.Podname, container.Nodename)
				if err != nil {
					log.Errorf("[Copy] Error when get node %s", cid)
					ch <- makeCopyMessage(cid, COPY_FAILED, "", "", err, nil)
					return
				}

				for _, path := range paths {
					wg.Add(1)
					go func(path string) {
						defer wg.Done()
						resp, stat, err := node.Engine.CopyFromContainer(ctx, container.ID, path)
						log.Debugf("[Copy] Docker cp stat: %v", stat)
						if err != nil {
							log.Errorf("[Copy] Error during CopyFromContainer: %v", err)
							ch <- makeCopyMessage(cid, COPY_FAILED, "", path, err, nil)
							return
						}
						ch <- makeCopyMessage(cid, COPY_OK, stat.Name, path, nil, resp)
					}(path)
				}
			}(cid, paths)
		}
		wg.Wait()
	}()
	return ch, nil
}

func makeCopyMessage(id, status, name, path string, err error, data io.ReadCloser) *types.CopyMessage {
	return &types.CopyMessage{
		ID:     id,
		Status: status,
		Name:   name,
		Path:   path,
		Error:  err,
		Data:   data,
	}
}
