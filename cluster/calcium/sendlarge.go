package calcium

import (
	"context"
	"io"
	"sync"

	"github.com/pkg/errors"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

// SendLargeFile send large files by stream to workload
func (c *Calcium) SendLargeFile(ctx context.Context, inputChan chan *types.SendLargeFileOptions) chan *types.SendMessage {
	resp := make(chan *types.SendMessage)
	wg := &sync.WaitGroup{}
	utils.SentryGo(func() {
		defer close(resp)
		senders := make(map[string]*workloadSender)
		// for each file
		for data := range inputChan {
			for _, id := range data.IDs {
				if _, ok := senders[id]; !ok {
					log.Debugf(ctx, "[SendLargeFile] create sender for %s", id)
					// for each container, let's create a new sender to send identical file chunk, each chunk will include the metadata of this file
					// we need to add `waitGroup out of the `newWorkloadSender` because we need to avoid `wg.Wait()` be executing before `wg.Add()`,
					// which will cause the goroutine in `c.newWorkloadSender` to be killed.
					wg.Add(1)
					sender := c.newWorkloadSender(ctx, id, resp, wg)
					senders[id] = sender
				}
				senders[id].send(data)
			}
		}
		for _, sender := range senders {
			sender.close()
		}
		wg.Wait()
	})
	return resp
}

type workloadSender struct {
	calcium *Calcium
	id      string
	wg      *sync.WaitGroup
	buffer  chan *types.SendLargeFileOptions
	resp    chan *types.SendMessage
}

func (c *Calcium) newWorkloadSender(ctx context.Context, ID string, resp chan *types.SendMessage, wg *sync.WaitGroup) *workloadSender {
	sender := &workloadSender{
		calcium: c,
		id:      ID,
		wg:      wg,
		buffer:  make(chan *types.SendLargeFileOptions, 10),
		resp:    resp,
	}
	utils.SentryGo(func() {
		var writer *io.PipeWriter
		curFile := ""
		for data := range sender.buffer {
			if curFile != "" && curFile != data.Dst {
				log.Warnf(ctx, "[newWorkloadExecutor] receive different files %s, %s", curFile, data.Dst)
				break
			}
			// ready to send
			if curFile == "" {
				log.Debugf(ctx, "[newWorkloadExecutor]Receive new file %s to %s", curFile, sender.id)
				curFile = data.Dst
				pr, pw := io.Pipe()
				writer = pw
				utils.SentryGo(func(ID, name string, size int64, content io.Reader, uid, gid int, mode int64) func() {
					return func() {
						defer wg.Done()
						if err := sender.calcium.withWorkloadLocked(ctx, ID, func(ctx context.Context, workload *types.Workload) error {
							err := errors.WithStack(workload.Engine.VirtualizationCopyChunkTo(ctx, ID, name, size, content, uid, gid, mode))
							resp <- &types.SendMessage{ID: ID, Path: name, Error: err}
							return nil
						}); err != nil {
							resp <- &types.SendMessage{ID: ID, Error: err}
						}
					}
				}(ID, curFile, data.Size, pr, data.UID, data.GID, data.Mode))
			}
			n, err := writer.Write(data.Chunk)
			if err != nil || n != len(data.Chunk) {
				log.Errorf(ctx, err, "[newWorkloadExecutor] send file to engine err, file = %s", curFile)
				break
			}
		}
		writer.Close()
	})
	return sender
}

func (s *workloadSender) send(chunk *types.SendLargeFileOptions) {
	s.buffer <- chunk
}

func (s *workloadSender) close() {
	close(s.buffer)
}
