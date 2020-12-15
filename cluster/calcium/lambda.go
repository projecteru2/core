package calcium

import (
	"context"
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

// RunAndWait implement lambda
func (c *Calcium) RunAndWait(ctx context.Context, opts *types.DeployOptions, inCh <-chan []byte) (<-chan *types.AttachWorkloadMessage, error) {
	if err := opts.Validate(); err != nil {
		return nil, err
	}
	opts.Lambda = true
	// count = 1 && OpenStdin
	if opts.OpenStdin && (opts.Count != 1 || opts.DeployStrategy != strategy.Auto) {
		log.Errorf("Count %d method %s", opts.Count, opts.DeployStrategy)
		return nil, types.ErrRunAndWaitCountOneWithStdin
	}

	createChan, err := c.CreateWorkload(ctx, opts)
	if err != nil {
		log.Errorf("[RunAndWait] Create workload error %s", err)
		return nil, err
	}

	runMsgCh := make(chan *types.AttachWorkloadMessage)
	wg := &sync.WaitGroup{}
	for message := range createChan {
		if message.Error != nil || message.WorkloadID == "" {
			log.Errorf("[RunAndWait] Create workload failed %s", message.Error)
			continue
		}

		lambda := func(message *types.CreateWorkloadMessage) {
			defer func() {
				if err := c.doRemoveWorkloadSync(context.Background(), []string{message.WorkloadID}); err != nil {
					log.Errorf("[RunAndWait] Remove lambda workload failed %v", err)
				} else {
					log.Infof("[RunAndWait] Workload %s finished and removed", utils.ShortID(message.WorkloadID))
				}
				wg.Done()
			}()

			workload, err := c.GetWorkload(ctx, message.WorkloadID)
			if err != nil {
				log.Errorf("[RunAndWait] Get workload failed %v", err)
				return
			}

			var outStream io.ReadCloser
			if outStream, err = workload.Engine.VirtualizationLogs(ctx, &enginetypes.VirtualizationLogStreamOptions{
				ID: message.WorkloadID, Follow: true, Stdout: true, Stderr: true}); err != nil {
				log.Errorf("[RunAndWait] Can't fetch log of workload %s error %v", message.WorkloadID, err)
				return
			}

			// use attach if use stdin
			if opts.OpenStdin {
				var inStream io.WriteCloser
				outStream, inStream, err = workload.Engine.VirtualizationAttach(ctx, message.WorkloadID, true, true)
				if err != nil {
					log.Errorf("[RunAndWait] Can't attach workload %s error %v", message.WorkloadID, err)
					return
				}

				processVirtualizationInStream(ctx, inStream, inCh, func(height, width uint) error {
					return workload.Engine.VirtualizationResize(ctx, message.WorkloadID, height, width)
				})
			}

			for data := range processVirtualizationOutStream(ctx, outStream) {
				runMsgCh <- &types.AttachWorkloadMessage{WorkloadID: message.WorkloadID, Data: data}
			}

			// wait and forward exitcode
			r, err := workload.Engine.VirtualizationWait(ctx, message.WorkloadID, "")
			if err != nil {
				log.Errorf("[RunAndWait] %s wait failed %v", utils.ShortID(message.WorkloadID), err)
				return
			}

			if r.Code != 0 {
				log.Errorf("[RunAndWait] %s run failed %s", utils.ShortID(message.WorkloadID), r.Message)
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
		log.Info("[RunAndWait] Finish run and wait for workloads")
	}()

	return runMsgCh, nil
}
