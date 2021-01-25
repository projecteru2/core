package calcium

import (
	"bufio"
	"context"
	"strconv"
	"sync"

	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
)

// ExecuteWorkload executes commands in running workloads
func (c *Calcium) ExecuteWorkload(ctx context.Context, opts *types.ExecuteWorkloadOptions, inCh <-chan []byte) chan *types.AttachWorkloadMessage {
	ch := make(chan *types.AttachWorkloadMessage)

	go func() {
		var err error
		chWg := sync.WaitGroup{}

		chWg.Add(1)
		defer func() {
			defer chWg.Done()
			if err != nil {
				ch <- &types.AttachWorkloadMessage{WorkloadID: opts.WorkloadID, Data: []byte(err.Error())}
			}
		}()

		go func() {
			defer close(ch)
			chWg.Wait()
		}()

		workload, err := c.GetWorkload(ctx, opts.WorkloadID)
		if err != nil {
			log.Errorf("[ExecuteWorkload] Failed to get wordload: %v", err)
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

		execID, stdoutStream, stderrStream, inStream, err := workload.Engine.Execute(ctx, opts.WorkloadID, execConfig)
		if err != nil {
			log.Errorf("[ExecuteWorkload] Failed to attach execID: %v", err)
			return
		}

		if opts.OpenStdin {
			processVirtualizationInStream(ctx, inStream, inCh, func(height, width uint) error {
				return workload.Engine.ExecResize(ctx, execID, height, width)
			})
		}

		scanSplitFunc, scanSplitBytes := bufio.ScanLines, []byte{'\n'}
		if execConfig.Tty {
			scanSplitFunc, scanSplitBytes = bufio.ScanBytes, []byte{}
		}

		streamWg := sync.WaitGroup{}

		chWg.Add(1)
		streamWg.Add(1)
		go func() {
			defer chWg.Done()
			defer streamWg.Done()
			for data := range processVirtualizationOutStream(ctx, stdoutStream, scanSplitFunc, scanSplitBytes) {
				ch <- &types.AttachWorkloadMessage{WorkloadID: opts.WorkloadID, Data: data, StdType: types.Stdout}
			}
		}()

		chWg.Add(1)
		streamWg.Add(1)
		go func() {
			defer chWg.Done()
			defer streamWg.Done()
			for data := range processVirtualizationOutStream(ctx, stderrStream, scanSplitFunc, scanSplitBytes) {
				ch <- &types.AttachWorkloadMessage{WorkloadID: opts.WorkloadID, Data: data, StdType: types.Stderr}
			}
		}()

		streamWg.Wait()
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
