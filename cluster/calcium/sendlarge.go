package calcium

import (
	"context"
	"github.com/pkg/errors"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
	"io"
	"sync"
)

// SendLargeFile send large files by stream to workload
func (c *Calcium) SendLargeFile(ctx context.Context, opts chan *types.SendLargeFileOptions) chan *types.SendMessage {
	resp := make(chan *types.SendMessage)
	wg := &sync.WaitGroup{}
	utils.SentryGo(func() {
		defer close(resp)
		handlers := make(map[string]chan *types.SendLargeFileOptions)
		for data := range opts {
			if err := data.Validate(); err != nil {
				continue
			}
			for _, id := range data.Ids {
				if _, ok := handlers[id]; !ok {
					log.Debugf(ctx, "[SendLargeFile] create handler for %s", id)
					handler := c.newWorkloadExecutor(ctx, id, resp, wg)
					handlers[id] = handler
				}
				handlers[id] <- data
			}
		}
		for _, handler := range handlers{
			close(handler)
		}
		wg.Wait()
	})
	return resp
}

func (c *Calcium) newWorkloadExecutor(ctx context.Context, ID string, resp chan *types.SendMessage, wg *sync.WaitGroup) chan *types.SendLargeFileOptions {
	input := make(chan *types.SendLargeFileOptions, 10)
	utils.SentryGo(func() {
		var writer *io.PipeWriter
		curFile := ""
		for data := range input {
			if curFile != "" && curFile != data.Dst {
				// todo 报错之后返回一下?
				log.Errorf(ctx, "[newWorkloadExecutor] receive different files %s, %s", curFile, data.Dst)
				break
			}
			if curFile == "" {
				wg.Add(1)
				log.Debugf(ctx, "[newWorkloadExecutor]Receive new file %s to %s", curFile, ID)
				curFile = data.Dst
				pr, pw := io.Pipe()
				writer = pw
				utils.SentryGo(func(ID, name string, size int64, content io.Reader, uid, gid int, mode int64) func() {
					return func() {
						defer wg.Done()
						if err := c.withWorkloadLocked(ctx, ID, func(ctx context.Context, workload *types.Workload) error {
							err := errors.WithStack(workload.Engine.VirtualizationCopyChunkTo(ctx, ID, name, size, content, uid, gid, mode))
							resp <- &types.SendMessage{ID: ID, Path: name, Error: err}
							return nil
						}); err != nil {
							resp <- &types.SendMessage{ID: ID, Error: err}
						}
					}
				}(ID, curFile, data.Size, pr, data.Uid, data.Gid, data.Mode))
			}
			writer.Write(data.Data)
		}
		writer.Close()
	})
	return input
}