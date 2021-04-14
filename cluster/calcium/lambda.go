package calcium

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strconv"
	"sync"

	"github.com/google/uuid"
	"github.com/pkg/errors"

	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/strategy"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
	"github.com/projecteru2/core/wal"
)

const (
	exitDataPrefix = "[exitcode] "
	labelLambdaID  = "LambdaID"
)

// RunAndWait implement lambda
func (c *Calcium) RunAndWait(ctx context.Context, opts *types.DeployOptions, inCh <-chan []byte) (<-chan *types.AttachWorkloadMessage, error) {
	logger := log.WithField("Calcium", "RunAndWait").WithField("opts", opts)
	if err := opts.Validate(); err != nil {
		return nil, logger.Err(err)
	}
	opts.Lambda = true
	// count = 1 && OpenStdin
	if opts.OpenStdin && (opts.Count != 1 || opts.DeployStrategy != strategy.Auto) {
		logger.Errorf("Count %d method %s", opts.Count, opts.DeployStrategy)
		return nil, errors.WithStack(types.ErrRunAndWaitCountOneWithStdin)
	}

	commit, err := c.walCreateLambda(opts)
	if err != nil {
		return nil, logger.Err(err)
	}
	createChan, err := c.CreateWorkload(ctx, opts)
	if err != nil {
		logger.Errorf("[RunAndWait] Create workload error %+v", err)
		return nil, err
	}

	var (
		runMsgCh      = make(chan *types.AttachWorkloadMessage)
		wg            = &sync.WaitGroup{}
		errorMessages = []*types.AttachWorkloadMessage{}
	)
	for message := range createChan {
		if message.Error != nil || message.WorkloadID == "" {
			logger.Errorf("[RunAndWait] Create workload failed %+v", message.Error)
			errorMessages = append(errorMessages, &types.AttachWorkloadMessage{
				WorkloadID:    "",
				Data:          []byte(fmt.Sprintf("Create workload failed %+v", message.Error)),
				StdStreamType: types.Stderr,
			})
			continue
		}

		lambda := func(message *types.CreateWorkloadMessage) {
			defer func() {
				if err := c.doRemoveWorkloadSync(context.TODO(), []string{message.WorkloadID}); err != nil {
					logger.Errorf("[RunAndWait] Remove lambda workload failed %+v", err)
				} else {
					log.Infof("[RunAndWait] Workload %s finished and removed", utils.ShortID(message.WorkloadID))
				}
				wg.Done()
			}()

			workload, err := c.GetWorkload(ctx, message.WorkloadID)
			if err != nil {
				logger.Errorf("[RunAndWait] Get workload failed %+v", err)
				errorMessages = append(errorMessages, &types.AttachWorkloadMessage{
					WorkloadID:    message.WorkloadID,
					Data:          []byte(fmt.Sprintf("Get workload %s failed %+v", message.WorkloadID, err)),
					StdStreamType: types.Stderr,
				})
				return
			}

			var stdout, stderr io.ReadCloser
			if stdout, stderr, err = workload.Engine.VirtualizationLogs(ctx, &enginetypes.VirtualizationLogStreamOptions{
				ID:     message.WorkloadID,
				Follow: true,
				Stdout: true,
				Stderr: true,
			}); err != nil {
				logger.Errorf("[RunAndWait] Can't fetch log of workload %s error %+v", message.WorkloadID, err)
				errorMessages = append(errorMessages, &types.AttachWorkloadMessage{
					WorkloadID:    message.WorkloadID,
					Data:          []byte(fmt.Sprintf("Fetch log for workload %s failed %+v", message.WorkloadID, err)),
					StdStreamType: types.Stderr,
				})
				return
			}

			splitFunc, split := bufio.ScanLines, byte('\n')

			// use attach if use stdin
			if opts.OpenStdin {
				var inStream io.WriteCloser
				stdout, stderr, inStream, err = workload.Engine.VirtualizationAttach(ctx, message.WorkloadID, true, true)
				if err != nil {
					logger.Errorf("[RunAndWait] Can't attach workload %s error %+v", message.WorkloadID, err)
					errorMessages = append(errorMessages, &types.AttachWorkloadMessage{
						WorkloadID:    message.WorkloadID,
						Data:          []byte(fmt.Sprintf("Attach to workload %s failed %+v", message.WorkloadID, err)),
						StdStreamType: types.Stderr,
					})
					return
				}

				processVirtualizationInStream(ctx, inStream, inCh, func(height, width uint) error {
					return workload.Engine.VirtualizationResize(ctx, message.WorkloadID, height, width)
				})

				splitFunc, split = bufio.ScanBytes, byte(0)
			}

			// return workload id as first normal message for lambda
			runMsgCh <- &types.AttachWorkloadMessage{
				WorkloadID:    message.WorkloadID,
				Data:          []byte(""),
				StdStreamType: types.Stdout,
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
				logger.Errorf("[RunAndWait] %s wait failed %+v", utils.ShortID(message.WorkloadID), err)
				errorMessages = append(errorMessages, &types.AttachWorkloadMessage{
					WorkloadID:    message.WorkloadID,
					Data:          []byte(fmt.Sprintf("Wait workload %s failed %+v", message.WorkloadID, err)),
					StdStreamType: types.Stderr,
				})
				return
			}

			if r.Code != 0 {
				logger.Errorf("[RunAndWait] %s run failed %s", utils.ShortID(message.WorkloadID), r.Message)
			}

			exitData := []byte(exitDataPrefix + strconv.Itoa(int(r.Code)))
			runMsgCh <- &types.AttachWorkloadMessage{WorkloadID: message.WorkloadID, Data: exitData}
		}

		wg.Add(1)
		go lambda(message)
	}

	go func() {
		defer close(runMsgCh)
		wg.Wait()
		if err := commit(); err != nil {
			logger.Errorf("[RunAndWait] Commit WAL %s failed: %v", eventCreateLambda, err)
		}

		for _, message := range errorMessages {
			runMsgCh <- message
		}
		log.Info("[RunAndWait] Finish run and wait for workloads")
	}()

	return runMsgCh, nil
}

func (c *Calcium) walCreateLambda(opts *types.DeployOptions) (wal.Commit, error) {
	uid, err := uuid.NewRandom()
	if err != nil {
		return nil, errors.WithStack(err)
	}

	lambdaID := uid.String()

	if opts.Labels != nil {
		opts.Labels[labelLambdaID] = lambdaID
	} else {
		opts.Labels = map[string]string{labelLambdaID: lambdaID}
	}

	return c.wal.logCreateLambda(opts)
}
