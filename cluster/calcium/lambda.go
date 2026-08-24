package calcium

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strconv"
	"sync"

	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/strategy"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

const exitDataPrefix = "[exitcode] "

func (c *Calcium) RunAndWait(ctx context.Context, opts *types.DeployOptions, inCh <-chan []byte) ([]string, <-chan *types.AttachWorkloadMessage, error) {
	workloadIDs := []string{}

	logger := log.WithFunc("calcium.RunAndWait").WithField("opts", opts)
	if err := opts.Validate(); err != nil {
		logger.Error(ctx, err)
		return workloadIDs, nil, err
	}
	opts.Lambda = true
	if opts.OpenStdin && (opts.Count != 1 || opts.DeployStrategy != strategy.Auto) {
		logger.Errorf(ctx, types.ErrRunAndWaitCountOneWithStdin, "count %d method %s", opts.Count, opts.DeployStrategy)
		return workloadIDs, nil, types.ErrRunAndWaitCountOneWithStdin
	}

	createChan, err := c.CreateWorkload(ctx, opts)
	if err != nil {
		logger.Error(ctx, err, "create workload")
		return workloadIDs, nil, err
	}

	var (
		runMsgCh = make(chan *types.AttachWorkloadMessage)
		wg       = &sync.WaitGroup{}
	)

	lambda := func(message *types.CreateWorkloadMessage) (attachMessage *types.AttachWorkloadMessage) {
		defer wg.Done()

		defer func() {
			runMsgCh <- attachMessage
		}()

		if message.Error != nil || message.WorkloadID == "" {
			logger.Error(ctx, message.Error, "create workload failed")
			return &types.AttachWorkloadMessage{
				WorkloadID:    "",
				Data:          []byte(fmt.Sprintf("Create workload failed %+v", message.Error)),
				StdStreamType: types.EruError,
			}
		}

		commit, err := c.wal.Log(eventCreateLambda, message.WorkloadID)
		if err != nil {
			logger.Error(ctx, err)
			return &types.AttachWorkloadMessage{
				WorkloadID:    message.WorkloadID,
				Data:          []byte(fmt.Sprintf("Create wal failed: %s, %+v", message.WorkloadID, err)),
				StdStreamType: types.EruError,
			}
		}
		defer func() {
			if commitErr := commit(); commitErr != nil {
				logger.Errorf(ctx, commitErr, "commit wal %s failed: %s", eventCreateLambda, message.WorkloadID)
			}
		}()

		defer func() {
			removeCtx, cancel := context.WithCancel(utils.NewInheritCtx(ctx))
			defer cancel()
			if removeErr := c.doRemoveWorkloadSync(removeCtx, []string{message.WorkloadID}); removeErr != nil {
				logger.Error(removeCtx, removeErr, "remove lambda workload failed")
			} else {
				logger.Infof(removeCtx, "workload %s finished and removed", utils.ShortID(message.WorkloadID))
			}
		}()

		workload, err := c.GetWorkload(ctx, message.WorkloadID)
		if err != nil {
			logger.Error(ctx, err, "get workload failed")
			return &types.AttachWorkloadMessage{
				WorkloadID:    message.WorkloadID,
				Data:          []byte(fmt.Sprintf("Get workload %s failed %+v", message.WorkloadID, err)),
				StdStreamType: types.EruError,
			}
		}

		var stdout, stderr io.ReadCloser
		if stdout, stderr, err = workload.Engine.VirtualizationLogs(ctx, &enginetypes.VirtualizationLogStreamOptions{
			ID:     message.WorkloadID,
			Follow: true,
			Stdout: true,
			Stderr: true,
		}); err != nil {
			logger.Errorf(ctx, err, "cannot fetch log of workload %s", message.WorkloadID)
			return &types.AttachWorkloadMessage{
				WorkloadID:    message.WorkloadID,
				Data:          []byte(fmt.Sprintf("Fetch log for workload %s failed %+v", message.WorkloadID, err)),
				StdStreamType: types.EruError,
			}
		}

		splitFunc, split := bufio.ScanLines, byte('\n')

		if opts.OpenStdin {
			var inStream io.WriteCloser
			stdout, stderr, inStream, err = workload.Engine.VirtualizationAttach(ctx, message.WorkloadID, true, true)
			if err != nil {
				logger.Errorf(ctx, err, "cannot attach workload %s", message.WorkloadID)
				return &types.AttachWorkloadMessage{
					WorkloadID:    message.WorkloadID,
					Data:          []byte(fmt.Sprintf("Attach to workload %s failed %+v", message.WorkloadID, err)),
					StdStreamType: types.EruError,
				}
			}

			c.processVirtualizationInStream(ctx, inStream, inCh, func(height, width uint) error {
				return workload.Engine.VirtualizationResize(ctx, message.WorkloadID, height, width)
			})

			splitFunc, split = bufio.ScanBytes, byte(0)
		}

		for m := range c.processStdStream(ctx, stdout, stderr, splitFunc, split) {
			runMsgCh <- &types.AttachWorkloadMessage{
				WorkloadID:    message.WorkloadID,
				Data:          m.Data,
				StdStreamType: m.StdStreamType,
			}
		}

		r, err := workload.Engine.VirtualizationWait(ctx, message.WorkloadID, "")
		if err != nil {
			logger.Errorf(ctx, err, "%s wait failed", utils.ShortID(message.WorkloadID))
			return &types.AttachWorkloadMessage{
				WorkloadID:    message.WorkloadID,
				Data:          []byte(fmt.Sprintf("Wait workload %s failed %+v", message.WorkloadID, err)),
				StdStreamType: types.EruError,
			}
		}

		if r.Code != 0 {
			logger.Warnf(ctx, "%s run failed: %s", utils.ShortID(message.WorkloadID), r.Message)
		}

		exitData := []byte(exitDataPrefix + strconv.Itoa(int(r.Code)))
		return &types.AttachWorkloadMessage{
			WorkloadID:    message.WorkloadID,
			Data:          exitData,
			StdStreamType: types.Stdout,
		}
	}

	for message := range createChan {
		workloadIDs = append(workloadIDs, message.WorkloadID)
		wg.Add(1)
		_ = c.pool.Invoke(func(msg *types.CreateWorkloadMessage) func() {
			return func() {
				lambda(msg)
			}
		}(message))
	}

	_ = c.pool.Invoke(func() {
		defer close(runMsgCh)
		wg.Wait()

		logger.Info(ctx, "finish run and wait for workloads")
	})

	return workloadIDs, runMsgCh, nil
}
