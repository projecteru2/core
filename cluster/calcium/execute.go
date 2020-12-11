package calcium

import (
	"context"
	"strconv"

	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
)

// ExecuteWorkload executes commands in running workloads
func (c *Calcium) ExecuteWorkload(ctx context.Context, opts *types.ExecuteWorkloadOptions, inCh <-chan []byte) chan *types.AttachWorkloadMessage {
	ch := make(chan *types.AttachWorkloadMessage)

	go func() {
		defer close(ch)
		var err error
		responses := []string{}
		defer func() {
			for _, resp := range responses {
				msg := &types.AttachWorkloadMessage{WorkloadID: opts.WorkloadID, Data: []byte(resp)}
				ch <- msg
			}
		}()

		workload, err := c.GetWorkload(ctx, opts.WorkloadID)
		if err != nil {
			responses = append(responses, err.Error())
			return
		}

		execConfig := &enginetypes.ExecConfig{
			Env:          opts.Envs,
			WorkingDir:   opts.Workdir,
			Cmd:          opts.Commands,
			AttachStderr: true,
			AttachStdout: true,
			AttachStdin:  opts.OpenStdin,
			Tty:          opts.OpenStdin,
			Detach:       false,
		}

		execID, outStream, inStream, err := workload.Engine.Execute(ctx, opts.WorkloadID, execConfig)
		if err != nil {
			log.Errorf("[ExecuteWorkload] Failed to attach execID: %v", err)
			return
		}

		if opts.OpenStdin {
			processVirtualizationInStream(ctx, inStream, inCh, func(height, width uint) error {
				return workload.Engine.ExecResize(ctx, execID, height, width)
			})
		}

		for data := range processVirtualizationOutStream(ctx, outStream) {
			ch <- &types.AttachWorkloadMessage{WorkloadID: opts.WorkloadID, Data: data}
		}

		execCode, err := workload.Engine.ExecExitCode(ctx, execID)
		if err != nil {
			log.Errorf("[ExecuteWorkload] Failed to get exitcode: %v", err)
			return
		}

		exitData := []byte(exitDataPrefix + strconv.Itoa(execCode))
		ch <- &types.AttachWorkloadMessage{WorkloadID: opts.WorkloadID, Data: exitData}
		log.Infof("[ExecuteWorkload] Execuate in workload %s complete", opts.WorkloadID)
		log.Infof("[ExecuteWorkload] %v", opts.Commands)
	}()

	return ch
}
