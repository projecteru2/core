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
		defer func() {
			_ = send(ctx, runMsgCh, attachMessage)
		}()

		if message.Error != nil || message.WorkloadID == "" {
			logger.Error(ctx, message.Error, "create workload failed")
			return newEruErrMsg("", "Create workload failed %v", message.Error)
		}

		commit, err := c.journal(ctx, logger, eventCreateLambda, message.WorkloadID)
		if err != nil {
			logger.Error(ctx, err)
			return newEruErrMsg(message.WorkloadID, "Create wal failed: %s, %v", message.WorkloadID, err)
		}
		defer func() {
			removeCtx := context.WithoutCancel(ctx)
			if removeErr := c.doRemoveWorkloadSync(removeCtx, []string{message.WorkloadID}); removeErr != nil {
				logger.Error(removeCtx, removeErr, "remove lambda workload failed")
				return
			}
			logger.Infof(removeCtx, "workload %s finished and removed", utils.ShortID(message.WorkloadID))
			commit()
		}()

		workload, err := c.GetWorkload(ctx, message.WorkloadID)
		if err != nil {
			logger.Error(ctx, err, "get workload failed")
			return newEruErrMsg(message.WorkloadID, "Get workload %s failed %v", message.WorkloadID, err)
		}

		var stdout, stderr io.ReadCloser
		splitFunc, split := bufio.ScanLines, byte('\n')

		if opts.OpenStdin {
			var inStream io.WriteCloser
			stdout, stderr, inStream, err = workload.Engine.VirtualizationAttach(ctx, message.WorkloadID, true, true)
			if err != nil {
				logger.Errorf(ctx, err, "cannot attach workload %s", message.WorkloadID)
				return newEruErrMsg(message.WorkloadID, "Attach to workload %s failed %v", message.WorkloadID, err)
			}

			c.processVirtualizationInStream(ctx, inStream, inCh, func(height, width uint) error {
				return workload.Engine.VirtualizationResize(ctx, message.WorkloadID, height, width)
			})

			splitFunc, split = bufio.ScanBytes, byte(0)
		} else if stdout, stderr, err = workload.Engine.VirtualizationLogs(ctx, &enginetypes.VirtualizationLogStreamOptions{
			ID:     message.WorkloadID,
			Follow: true,
			Stdout: true,
			Stderr: true,
		}); err != nil {
			logger.Errorf(ctx, err, "cannot fetch log of workload %s", message.WorkloadID)
			return newEruErrMsg(message.WorkloadID, "Fetch log for workload %s failed %v", message.WorkloadID, err)
		}

		for m := range c.processStdStream(ctx, stdout, stderr, splitFunc, split) {
			if send(ctx, runMsgCh, &types.AttachWorkloadMessage{
				WorkloadID:    message.WorkloadID,
				Data:          m.Data,
				StdStreamType: m.StdStreamType,
			}) != nil {
				break
			}
		}

		r, err := workload.Engine.VirtualizationWait(ctx, message.WorkloadID, "")
		if err != nil {
			logger.Errorf(ctx, err, "%s wait failed", utils.ShortID(message.WorkloadID))
			return newEruErrMsg(message.WorkloadID, "Wait workload %s failed %v", message.WorkloadID, err)
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
		wg.Go(func() {
			defer log.SentryDefer()
			lambda(message)
		})
	}

	utils.SentryGo(func() {
		defer close(runMsgCh)
		wg.Wait()

		logger.Info(ctx, "finish run and wait for workloads")
	})

	return workloadIDs, runMsgCh, nil
}

func newEruErrMsg(workloadID, format string, args ...any) *types.AttachWorkloadMessage {
	return &types.AttachWorkloadMessage{
		WorkloadID:    workloadID,
		Data:          []byte(fmt.Sprintf(format, args...)),
		StdStreamType: types.EruError,
	}
}
