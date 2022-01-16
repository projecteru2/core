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
	logger := log.WithField("Calcium", "SendLargeFile").WithField("opts", opts)
	utils.SentryGo(func() {
		defer close(resp)
		wg := &sync.WaitGroup{}

		writerMap := make(map[string]map[string]*io.PipeWriter)
		for data := range opts {
			if err := data.Validate(); err != nil {
				continue
			}

			dst := data.FileMetadataOptions.Dst
			for _, id := range data.FileMetadataOptions.Ids {
				if _, ok := writerMap[id]; !ok {
					writerMap[id] = make(map[string]*io.PipeWriter)
				}
				if _, ok := writerMap[id][dst]; !ok {
					pr, pw := io.Pipe()
					writerMap[id][dst] = pw
					utils.SentryGo(func(id string) func() {
						return func() {
							defer wg.Done()
							if err := c.withWorkloadLocked(ctx, id, func(ctx context.Context, workload *types.Workload) error {
								err := c.doSendChunkFileToWorkload(ctx, workload.Engine, workload.ID, dst, pr, data.Uid, data.Gid, data.Mode)
								resp <- &types.SendMessage{ID: id, Path: dst, Error: logger.Err(ctx, err)}
								return nil
							}); err != nil {
								resp <- &types.SendMessage{ID: id, Error: logger.Err(ctx, err)}
							}
						}
					}(id))
				}
				writerMap[id][dst].Write(data.Data)
			}
		}
	})
	return nil
}

func (c *Calcium) doSendChunkFileToWorkload(ctx context.Context, engine engine.API, ID, name string, content io.Reader, uid, gid int, mode int64) error {
	log.Infof(ctx, "[doSendChunkFileToWorkload] Send file to %s:%s", ID, name)
	return errors.WithStack(engine.VirtualizationCopyChunkTo(ctx, ID, name, content, uid, gid, mode))
}