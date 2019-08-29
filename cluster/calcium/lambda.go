package calcium

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"

	"github.com/projecteru2/core/cluster"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
	log "github.com/sirupsen/logrus"
)

var winchCommand = []byte{0xf, 0xa}

type window struct {
	Height uint `json:"Row"`
	Width  uint `json:"Col"`
}

//RunAndWait implement lambda
func (c *Calcium) RunAndWait(ctx context.Context, opts *types.DeployOptions, stdinCh <-chan []byte) (<-chan *types.RunAndWaitMessage, error) {

	// 强制为 json-file 输出
	opts.Entrypoint.Log = &types.LogConfig{Type: "json-file"}

	// count = 1 && OpenStdin
	if opts.OpenStdin && (opts.Count != 1 || opts.DeployMethod != cluster.DeployAuto) {
		log.Errorf("Count %d method %s", opts.Count, opts.DeployMethod)
		err := types.ErrRunAndWaitCountOneWithStdin
		return nil, err
	}

	// 不能让 context 作祟
	createChan, err := c.CreateContainer(context.Background(), opts)
	if err != nil {
		log.Errorf("[RunAndWait] Create container error %s", err)
		return nil, err
	}

	runAndWait := c.fetchLog
	if opts.OpenStdin {
		runAndWait = c.attach
	}

	runMsgCh := make(chan *types.RunAndWaitMessage)
	wg := &sync.WaitGroup{}
	for message := range createChan {
		if !message.Success || message.ContainerID == "" {
			log.Errorf("[RunAndWait] Create container error, %s", message.Error)
			continue
		}

		wg.Add(1)
		go func(message *types.CreateContainerMessage) {
			defer func() {
				wg.Done()
				log.Infof("[runAndWait] Container %s finished and removed", utils.ShortID(message.ContainerID))
				c.doRemoveContainerSync(context.Background(), []string{message.ContainerID})
			}()

			node, err := c.GetNode(ctx, message.Podname, message.Nodename)
			if err != nil {
				log.Errorf("[runAndWait] Can't find node, %v", err)
				return
			}

			// attach if stdin opened, or just fetch log
			outputCh := runAndWait(ctx, node, message.ContainerID, stdinCh)
			for output := range outputCh {
				runMsgCh <- &types.RunAndWaitMessage{ContainerID: message.ContainerID, Data: output}
			}

			// wait and forward exitcode
			r, err := node.Engine.VirtualizationWait(ctx, message.ContainerID, "")
			if err != nil {
				log.Errorf("[runAndWait] %s runs failed: %v", utils.ShortID(message.ContainerID), err)
				return
			}

			if r.Code != 0 {
				log.Errorf("[RunAndWait] %s run failed %s", utils.ShortID(message.ContainerID), r.Message)
			}
			exitData := []byte(fmt.Sprintf("[exitcode] %d", r.Code))
			runMsgCh <- &types.RunAndWaitMessage{ContainerID: message.ContainerID, Data: exitData}
		}(message)
	}

	go func() {
		wg.Wait()
		log.Info("[RunAndWait] Finish run and wait for containers")
		close(runMsgCh)
	}()

	return runMsgCh, nil
}

func (c *Calcium) attach(
	ctx context.Context,
	node *types.Node,
	containerID string,
	stdinCh <-chan []byte,
) (outputCh chan []byte) {

	output, input, err := node.Engine.VirtualizationAttach(ctx, containerID, true, true)
	if err != nil {
		log.Errorf("Can't attach container %s: %v", containerID, err)
		return
	}

	// copy stdin IO
	go func() {
		defer input.Close()

		w := &window{}
		for cmd := range stdinCh {
			if bytes.HasPrefix(cmd, winchCommand) {
				log.Debugf("SIGWINCH: %q", cmd)
				if err := json.Unmarshal(cmd[len(winchCommand):], w); err != nil {
					log.Errorf("Recv winch error: %v", err)
				} else {
					if err := node.Engine.VirtualizationResize(ctx, containerID, w.Height, w.Width); err != nil {
						log.Errorf("Resize window error: %v", err)
					}
				}
				continue
			}

			for _, b := range cmd {
				_, err := input.Write([]byte{b})
				if err != nil {
					log.Errorf("failed to write input to virtual unit: %v", err)
					return
				}
			}
		}
	}()

	// copy stdout IO
	outputCh = make(chan []byte)
	go func() {
		defer output.Close()
		defer close(outputCh)
		buf := make([]byte, 1024)
		for {
			n, err := output.Read(buf)
			if n > 0 {
				outputCh <- buf[:n]
			}
			if err != nil {
				if err == io.EOF {
					return
				}
				log.Errorf("failed to read output from virtual unit: %v", err)
				return
			}
		}
	}()

	return
}

func (c *Calcium) fetchLog(
	ctx context.Context,
	node *types.Node,
	containerID string,
	stdinCh <-chan []byte,
) (outputCh chan []byte) {
	output, err := node.Engine.VirtualizationLogs(ctx, containerID, true, true, true)
	if err != nil {
		log.Errorf("Can't fetch log of container %s: %v", containerID, err)
		return
	}

	outputCh = make(chan []byte)
	go func() {
		defer close(outputCh)
		scanner := bufio.NewScanner(output)
		log.Debug("start parsing output")
		for scanner.Scan() {
			data := scanner.Bytes()
			log.Debugf("container log: %q", data)
			outputCh <- data
		}

		if err := scanner.Err(); err != nil {
			log.Errorf("parse stdout failed: %v", err)
		}
	}()
	return
}
