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

	"github.com/pkg/errors"
)

const (
	exitDataPrefix = "[exitcode] "
)

// RunAndWait implement lambda
func (c *Calcium) RunAndWait(ctx context.Context, opts *types.DeployOptions, inCh <-chan []byte) ([]string, <-chan *types.AttachWorkloadMessage, error) {
	workloadIDs := []string{}

	logger := log.WithField("Calcium", "RunAndWait").WithField("opts", opts)
	if err := opts.Validate(); err != nil {
		return workloadIDs, nil, logger.Err(ctx, err)
	}
	opts.Lambda = true
	// count = 1 && OpenStdin
	if opts.OpenStdin && (opts.Count != 1 || opts.DeployStrategy != strategy.Auto) {
		logger.Errorf(ctx, "Count %d method %s", opts.Count, opts.DeployStrategy)
		return workloadIDs, nil, errors.WithStack(types.ErrRunAndWaitCountOneWithStdin)
	}

	createChan, err := c.CreateWorkload(ctx, opts)
	if err != nil {
		logger.Errorf(ctx, "[RunAndWait] Create workload error %+v", err)
		return workloadIDs, nil, err
	}

	var (
		runMsgCh = make(chan *types.AttachWorkloadMessage)
		wg       = &sync.WaitGroup{}
	)

	lambda := func(message *types.CreateWorkloadMessage) (attachMessage *types.AttachWorkloadMessage) {
		// should Done this waitgroup anyway
		defer wg.Done()

		defer func() {
			runMsgCh <- attachMessage
		}()

		// if workload is empty, which means error occurred when created workload
		// we don't need to remove this non-existing workload
		// so just send the error message and return
		if message.Error != nil || message.WorkloadID == "" {
			logger.Errorf(ctx, "[RunAndWait] Create workload failed %+v", message.Error)
			return &types.AttachWorkloadMessage{
				WorkloadID:    "",
				Data:          []byte(fmt.Sprintf("Create workload failed %+v", errors.Unwrap(message.Error))),
				StdStreamType: types.EruError,
			}
		}

		commit, err := c.wal.Log(eventCreateLambda, message.WorkloadID)
		if err != nil {
			return &types.AttachWorkloadMessage{
				WorkloadID:    message.WorkloadID,
				Data:          []byte(fmt.Sprintf("Create wal failed: %s, %+v", message.WorkloadID, logger.Err(ctx, err))),
				StdStreamType: types.EruError,
			}
		}
		defer func() {
			if err := commit(); err != nil {
				logger.Errorf(ctx, "[RunAndWait] Commit WAL %s failed: %s, %v", eventCreateLambda, message.WorkloadID, err)
			}
		}()

		// the workload should be removed if it exists
		// no matter the workload exits successfully or not
		defer func() {
			ctx, cancel := context.WithCancel(utils.InheritTracingInfo(ctx, context.TODO()))
			defer cancel()
			if err := c.doRemoveWorkloadSync(ctx, []string{message.WorkloadID}); err != nil {
				logger.Errorf(ctx, "[RunAndWait] Remove lambda workload failed %+v", err)
			} else {
				log.Infof(ctx, "[RunAndWait] Workload %s finished and removed", utils.ShortID(message.WorkloadID))
			}
		}()

		// if we can't get the workload but message has workload field
		// this is weird, we return the error directly and try to delete data
		workload, err := c.GetWorkload(ctx, message.WorkloadID)
		if err != nil {
			logger.Errorf(ctx, "[RunAndWait] Get workload failed %+v", err)
			return &types.AttachWorkloadMessage{
				WorkloadID:    message.WorkloadID,
				Data:          []byte(fmt.Sprintf("Get workload %s failed %+v", message.WorkloadID, errors.Unwrap(err))),
				StdStreamType: types.EruError,
			}
		}

		// for other cases, we have the workload and it works fine
		// then we need to forward log, and finally delete the workload
		// of course all the error messages will be sent back
		var stdout, stderr io.ReadCloser
		if stdout, stderr, err = workload.Engine.VirtualizationLogs(ctx, &enginetypes.VirtualizationLogStreamOptions{
			ID:     message.WorkloadID,
			Follow: true,
			Stdout: true,
			Stderr: true,
		}); err != nil {
			logger.Errorf(ctx, "[RunAndWait] Can't fetch log of workload %s error %+v", message.WorkloadID, err)
			return &types.AttachWorkloadMessage{
				WorkloadID:    message.WorkloadID,
				Data:          []byte(fmt.Sprintf("Fetch log for workload %s failed %+v", message.WorkloadID, errors.Unwrap(err))),
				StdStreamType: types.EruError,
			}
		}

		splitFunc, split := bufio.ScanLines, byte('\n')

		// use attach if use stdin
		if opts.OpenStdin {
			var inStream io.WriteCloser
			stdout, stderr, inStream, err = workload.Engine.VirtualizationAttach(ctx, message.WorkloadID, true, true)
			if err != nil {
				logger.Errorf(ctx, "[RunAndWait] Can't attach workload %s error %+v", message.WorkloadID, err)
				return &types.AttachWorkloadMessage{
					WorkloadID:    message.WorkloadID,
					Data:          []byte(fmt.Sprintf("Attach to workload %s failed %+v", message.WorkloadID, errors.Unwrap(err))),
					StdStreamType: types.EruError,
				}
			}

			processVirtualizationInStream(ctx, inStream, inCh, func(height, width uint) error {
				return workload.Engine.VirtualizationResize(ctx, message.WorkloadID, height, width)
			})

			splitFunc, split = bufio.ScanBytes, byte(0)
		}

		for m := range processStdStream(ctx, stdout, stderr, splitFunc, split) {
			runMsgCh <- &types.AttachWorkloadMessage{
				WorkloadID:    message.WorkloadID,
				Data:          m.Data,
				StdStreamType: m.StdStreamType,
			}
		}

		// wait and forward exitcode
		r, err := workload.Engine.VirtualizationWait(ctx, message.WorkloadID, "")
		if err != nil {
			logger.Errorf(ctx, "[RunAndWait] %s wait failed %+v", utils.ShortID(message.WorkloadID), err)
			return &types.AttachWorkloadMessage{
				WorkloadID:    message.WorkloadID,
				Data:          []byte(fmt.Sprintf("Wait workload %s failed %+v", message.WorkloadID, errors.Unwrap(err))),
				StdStreamType: types.EruError,
			}
		}

		if r.Code != 0 {
			logger.Errorf(ctx, "[RunAndWait] %s run failed %s", utils.ShortID(message.WorkloadID), r.Message)
		}

		exitData := []byte(exitDataPrefix + strconv.Itoa(int(r.Code)))
		return &types.AttachWorkloadMessage{
			WorkloadID:    message.WorkloadID,
			Data:          exitData,
			StdStreamType: types.Stdout,
		}
	}

	for message := range createChan {
		// iterate over messages to store workload ids
		workloadIDs = append(workloadIDs, message.WorkloadID)
		wg.Add(1)
		utils.SentryGo(func(msg *types.CreateWorkloadMessage) func() {
			return func() {
				lambda(msg)
			}
		}(message))
	}

	utils.SentryGo(func() {
		defer close(runMsgCh)
		wg.Wait()

		log.Info("[RunAndWait] Finish run and wait for workloads")
	})

	return workloadIDs, runMsgCh, nil
}
