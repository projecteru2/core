package calcium

import (
	"context"
	"io"
	"strconv"
	"sync"

	"github.com/projecteru2/core/cluster"
	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
	log "github.com/sirupsen/logrus"
)

const exitDataPrefix = "[exitcode] "

// RunAndWait implement lambda
func (c *Calcium) RunAndWait(ctx context.Context, opts *types.DeployOptions, inCh <-chan []byte) (<-chan *types.AttachContainerMessage, error) {
	opts.Lambda = true
	// count = 1 && OpenStdin
	if opts.OpenStdin && (opts.Count != 1 || opts.DeployMethod != cluster.DeployAuto) {
		log.Errorf("Count %d method %s", opts.Count, opts.DeployMethod)
		return nil, types.ErrRunAndWaitCountOneWithStdin
	}

	createChan, err := c.CreateContainer(ctx, opts)
	if err != nil {
		log.Errorf("[RunAndWait] Create container error %s", err)
		return nil, err
	}

	runMsgCh := make(chan *types.AttachContainerMessage)
	wg := &sync.WaitGroup{}
	for message := range createChan {
		if message.Error != nil || message.ContainerID == "" {
			log.Errorf("[RunAndWait] Create container failed %s", message.Error)
			continue
		}

		lambda := func(message *types.CreateContainerMessage) {
			defer func() {
				if err := c.doRemoveContainerSync(context.Background(), []string{message.ContainerID}); err != nil {
					log.Errorf("[RunAndWait] Remove lambda container failed %v", err)
				} else {
					log.Infof("[RunAndWait] Container %s finished and removed", utils.ShortID(message.ContainerID))
				}
				wg.Done()
			}()

			container, err := c.GetContainer(ctx, message.ContainerID)
			if err != nil {
				log.Errorf("[RunAndWait] Get container failed %v", err)
				return
			}

			var outStream io.ReadCloser
			if outStream, err = container.Engine.VirtualizationLogs(ctx, &enginetypes.VirtualizationLogStreamOptions{
				ID: message.ContainerID, Follow: true, Stdout: true, Stderr: true}); err != nil {
				log.Errorf("[RunAndWait] Can't fetch log of container %s error %v", message.ContainerID, err)
				return
			}

			// use attach if use stdin
			if opts.OpenStdin {
				var inStream io.WriteCloser
				outStream, inStream, err = container.Engine.VirtualizationAttach(ctx, message.ContainerID, true, true)
				if err != nil {
					log.Errorf("[RunAndWait] Can't attach container %s error %v", message.ContainerID, err)
					return
				}

				processVirtualizationInStream(ctx, inStream, inCh, func(height, width uint) error {
					return container.Engine.VirtualizationResize(ctx, message.ContainerID, height, width)
				})
			}

			for data := range processVirtualizationOutStream(ctx, outStream) {
				runMsgCh <- &types.AttachContainerMessage{ContainerID: message.ContainerID, Data: data}
			}

			// wait and forward exitcode
			r, err := container.Engine.VirtualizationWait(ctx, message.ContainerID, "")
			if err != nil {
				log.Errorf("[RunAndWait] %s wait failed %v", utils.ShortID(message.ContainerID), err)
				return
			}

			if r.Code != 0 {
				log.Errorf("[RunAndWait] %s run failed %s", utils.ShortID(message.ContainerID), r.Message)
			}

			exitData := []byte(exitDataPrefix + strconv.Itoa(int(r.Code)))
			runMsgCh <- &types.AttachContainerMessage{ContainerID: message.ContainerID, Data: exitData}
		}

		wg.Add(1)
		go lambda(message)
	}

	go func() {
		defer close(runMsgCh)
		wg.Wait()
		log.Info("[RunAndWait] Finish run and wait for containers")
	}()

	return runMsgCh, nil
}
