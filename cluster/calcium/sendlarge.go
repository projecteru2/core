package calcium

import (
	"context"
	"io"
	"sync"

	"github.com/cockroachdb/errors"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

func (c *Calcium) SendLargeFile(ctx context.Context, inputChan chan *types.SendLargeFileOptions) chan *types.SendMessage {
	logger := log.WithFunc("calcium.SendLargeFile")
	resp := make(chan *types.SendMessage)
	wg := &sync.WaitGroup{}
	utils.SentryGo(func() {
		defer close(resp)
		senders := make(map[string]*workloadSender)
		for data := range inputChan {
			for _, id := range data.IDs {
				if _, ok := senders[id]; !ok {
					logger.Debugf(ctx, "create sender for %s", id)
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
	buffer  chan *types.SendLargeFileOptions
}

func (s *workloadSender) send(chunk *types.SendLargeFileOptions) {
	s.buffer <- chunk
}

func (s *workloadSender) close() {
	close(s.buffer)
}

func (c *Calcium) newWorkloadSender(ctx context.Context, ID string, resp chan *types.SendMessage, wg *sync.WaitGroup) *workloadSender {
	logger := log.WithFunc("calcium.newWorkloadSender")
	sender := &workloadSender{
		calcium: c,
		id:      ID,
		buffer:  make(chan *types.SendLargeFileOptions, 10),
	}
	utils.SentryGo(func() {
		defer wg.Done()
		var writer *io.PipeWriter
		curFile := ""
		for data := range sender.buffer {
			if curFile != "" && curFile != data.Dst {
				logger.Warnf(ctx, "receive different files %s, %s", curFile, data.Dst)
				break
			}
			if curFile == "" {
				logger.Debugf(ctx, "receive new file %s to %s", data.Dst, sender.id)
				curFile = data.Dst
				pr, pw := io.Pipe()
				writer = pw
				wg.Add(1)
				utils.SentryGo(func() {
					defer wg.Done()
					defer func() { _ = pr.Close() }()
					if err := sender.calcium.withWorkloadLocked(ctx, ID, false, func(ctx context.Context, workload *types.Workload) error {
						err := errors.WithStack(workload.Engine.VirtualizationCopyChunkTo(ctx, ID, curFile, data.Size, pr, data.UID, data.GID, data.Mode))
						resp <- &types.SendMessage{ID: ID, Path: curFile, Error: err}
						return nil
					}); err != nil {
						resp <- &types.SendMessage{ID: ID, Error: err}
					}
				})
			}
			n, err := writer.Write(data.Chunk)
			if err != nil || n != len(data.Chunk) {
				logger.Errorf(ctx, err, "send file to engine err, file = %s", curFile)
				break
			}
		}
		_ = writer.Close()
	})
	return sender
}
