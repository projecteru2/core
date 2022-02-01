package calcium

import (
	"context"
	"github.com/pkg/errors"
	"github.com/projecteru2/core/engine"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
	"io"
	"sync"
)

// SendLargeFile send large files by stream to workload
func (c *Calcium) SendLargeFile(ctx context.Context, opts chan *types.SendLargeFileOptions, resp chan *types.SendMessage) error {
	//logger := log.WithField("Calcium", "SendLargeFile").WithField("opts", opts)
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
	return nil
}

func (c *Calcium) newWorkloadExecutor(ctx context.Context, ID string, resp chan *types.SendMessage, wg *sync.WaitGroup) chan *types.SendLargeFileOptions {
	input := make(chan *types.SendLargeFileOptions, 10)
	utils.SentryGo(func() {
		writerMap := make(map[string]*io.PipeWriter)
		preFile := ""
		for data := range input {
			curFile := data.Dst
			if preFile == "" || preFile != curFile {
				wg.Add(1)
				log.Debugf(ctx, "[newWorkloadExecutor]Receive new file file %s to %s", curFile, ID)
				if preFile != "" {
					writerMap[preFile].Close()
					delete(writerMap, preFile)
				}
				preFile = curFile
				pr, pw := io.Pipe()
				writerMap[curFile] = pw
				utils.SentryGo(func(ID, name string, size int64, content io.Reader, uid, gid int, mode int64) func() {
					return func() {
						defer wg.Done()
						if err := c.withWorkloadLocked(ctx, ID, func(ctx context.Context, workload *types.Workload) error {
							err := c.doSendChunkFileToWorkload(ctx, workload.Engine, workload.ID, name, size, content, uid, gid, mode)
							resp <- &types.SendMessage{ID: ID, Path: name, Error: err}
							return nil
						}); err != nil {
							log.Debugf(ctx, "[newWorkloadExecutor]container %s exec %s err, err=%v", ID, name, err)
							resp <- &types.SendMessage{ID: ID, Error: err}
						}
					}
				}(ID, curFile, data.Size, pr, data.Uid, data.Gid, data.Mode))
			}
			writerMap[curFile].Write(data.Data)
		}
		for _, pw := range writerMap {
			pw.Close()
		}
	})
	return input
}

func (c *Calcium) doSendChunkFileToWorkload(ctx context.Context, engine engine.API, ID, name string, size int64, content io.Reader, uid, gid int, mode int64) error {
	log.Infof(ctx, "[doSendChunkFileToWorkload] Send file to %s:%s", ID, name)
	return errors.WithStack(engine.VirtualizationCopyChunkTo(ctx, ID, name, size, content, uid, gid, mode))
}