package calcium

import (
	"context"
	"fmt"
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
func (c *Calcium) RunAndWait(ctx context.Context, opts *types.DeployOptions, inCh <-chan []byte) (<-chan *types.RunAndWaitMessage, error) {

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

	runMsgCh := make(chan *types.RunAndWaitMessage)
	wg := &sync.WaitGroup{}
	for message := range createChan {
		if !message.Success || message.ContainerID == "" {
			log.Errorf("[RunAndWait] Create container error, %s", message.Error)
			continue
		}

		runAndWait := func(message *types.CreateContainerMessage) {
			defer func() {
				wg.Done()
				c.doRemoveContainerSync(context.Background(), []string{message.ContainerID})
				log.Infof("[runAndWait] Container %s finished and removed", utils.ShortID(message.ContainerID))
			}()

			node, err := c.GetNode(ctx, message.Podname, message.Nodename)
			if err != nil {
				log.Errorf("[runAndWait] Can't find node, %v", err)
				return
			}

			outStream, inStream, err := node.Engine.VirtualizationAttach(ctx, message.ContainerID, true, true)
			if err != nil {
				log.Errorf("[runAndWait] Can't attach container %s: %v", message.ContainerID, err)
				return
			}
			if !opts.OpenStdin {
				outStream, err = node.Engine.VirtualizationLogs(ctx, message.ContainerID, true, true, true)
				if err != nil {
					log.Errorf("[runAndWait] Can't fetch log of container %s: %v", message.ContainerID, err)
					return
				}
			}

			ProcessVirtualizationInStream(ctx, inStream, inCh, func(height, width uint) error {
				return node.Engine.VirtualizationResize(ctx, message.ContainerID, height, width)
			})
			for data := range ProcessVirtualizationOutStream(ctx, outStream) {
				runMsgCh <- &types.RunAndWaitMessage{ContainerID: message.ContainerID, Data: data}
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
			return
		}

		wg.Add(1)
		go runAndWait(message)
	}

	go func() {
		wg.Wait()
		log.Info("[RunAndWait] Finish run and wait for containers")
		close(runMsgCh)
	}()

	return runMsgCh, nil
}
