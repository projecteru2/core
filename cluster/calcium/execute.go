package calcium

import (
	"bufio"
	"context"
	"strconv"

	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

// ExecuteWorkload executes commands in running workloads
func (c *Calcium) ExecuteWorkload(ctx context.Context, opts *types.ExecuteWorkloadOptions, inCh <-chan []byte) chan *types.AttachWorkloadMessage {
	logger := log.WithField("Calcium", "ExecuteWorkload").WithField("opts", opts)
	ch := make(chan *types.AttachWorkloadMessage)

	utils.SentryGo(func() {
		var err error

		defer func() {
			if err != nil {
				ch <- &types.AttachWorkloadMessage{WorkloadID: opts.WorkloadID, Data: []byte(err.Error())}
			}
			close(ch)
		}()

		workload, err := c.GetWorkload(ctx, opts.WorkloadID)
		if err != nil {
			logger.Errorf(ctx, "[ExecuteWorkload] Failed to get wordload: %+v", err)
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

		execID, stdout, stderr, inStream, err := workload.Engine.Execute(ctx, opts.WorkloadID, execConfig)
		if err != nil {
			logger.Errorf(ctx, "[ExecuteWorkload] Failed to attach execID: %+v", err)
			return
		}

		splitFunc, split := bufio.ScanLines, byte('\n')
		if opts.OpenStdin {
			processVirtualizationInStream(ctx, inStream, inCh, func(height, width uint) error {
				return workload.Engine.ExecResize(ctx, execID, height, width)
			})
			splitFunc, split = bufio.ScanBytes, byte(0)
		}

		for m := range processStdStream(ctx, stdout, stderr, splitFunc, split) {
			ch <- &types.AttachWorkloadMessage{WorkloadID: opts.WorkloadID, Data: m.Data, StdStreamType: m.StdStreamType}
		}

		execCode, err := workload.Engine.ExecExitCode(ctx, execID)
		if err != nil {
			logger.Errorf(ctx, "[ExecuteWorkload] Failed to get exitcode: %+v", err)
			return
		}

		exitData := []byte(exitDataPrefix + strconv.Itoa(execCode))
		ch <- &types.AttachWorkloadMessage{WorkloadID: opts.WorkloadID, Data: exitData}
		log.Infof(ctx, "[ExecuteWorkload] Execuate in workload %s complete", opts.WorkloadID)
		log.Infof(ctx, "[ExecuteWorkload] %v", opts.Commands)
	})

	return ch
}
