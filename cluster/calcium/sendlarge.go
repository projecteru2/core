package calcium

import (
	"context"
	"github.com/pkg/errors"
	"github.com/projecteru2/core/engine"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
	"github.com/sirupsen/logrus"
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
			logrus.Debugf("[SendLargeFile] receive some msg, len: %d", len(data.Data))
			if err := data.Validate(); err != nil {
				continue
			}

			dst := data.FileMetadataOptions.Dst
			for _, id := range data.FileMetadataOptions.Ids {
				if _, ok := writerMap[id]; !ok {
					logrus.Debugf("[SendLargeFile] gen writerMap[%s]", id)
					writerMap[id] = make(map[string]*io.PipeWriter)
				}
				if _, ok := writerMap[id][dst]; !ok {
					wg.Add(1)
					pr, pw := io.Pipe()
					writerMap[id][dst] = pw
					utils.SentryGo(func(id string) func() {
						return func() {
							defer wg.Done()
							logrus.Debugf("[SendLargeFile] gen goroutine for id:%s", id)
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
				logrus.Debugf("[SendLargeFile] send somethine id: %s; dst: %s", id, dst)
				writerMap[id][dst].Write(data.Data)
			}
		}
		logrus.Debugf("[SendLargeFile] send to the end")
		for _, m := range writerMap {
			for _, w := range m{
				err := w.Close()
				if err != nil {
					logrus.Debugf("[SendLargeFile] close writer error: %v", err)
				}
			}
		}
		wg.Wait()
	})
	return nil
}

func (c *Calcium) doSendChunkFileToWorkload(ctx context.Context, engine engine.API, ID, name string, content io.Reader, uid, gid int, mode int64) error {
	log.Infof(ctx, "[doSendChunkFileToWorkload] Send file to %s:%s", ID, name)
	return errors.WithStack(engine.VirtualizationCopyChunkTo(ctx, ID, name, content, uid, gid, mode))
}