package calcium

import (
	"context"
	"fmt"
	"sync"

	"github.com/projecteru2/core/types"
	log "github.com/sirupsen/logrus"
)

// RemoveImage remove images
func (c *Calcium) RemoveImage(ctx context.Context, podname, nodename string, images []string, prune bool) (chan *types.RemoveImageMessage, error) {
	ch := make(chan *types.RemoveImageMessage)

	var err error
	nodes := []*types.Node{}
	if nodename != "" {
		n, err := c.GetNode(ctx, podname, nodename)
		if err != nil {
			return ch, err
		}
		nodes = append(nodes, n)
	} else {
		nodes, err = c.store.GetNodesByPod(ctx, podname)
		if err != nil {
			return ch, err
		}
	}

	go func() {
		defer close(ch)
		wg := sync.WaitGroup{}
		for _, node := range nodes {
			wg.Add(1)
			go func(node *types.Node) {
				defer wg.Done()
				for _, image := range images {
					m := &types.RemoveImageMessage{
						Success:  false,
						Image:    image,
						Messages: []string{},
					}
					if removeItems, err := node.Engine.ImageRemove(ctx, image, false, true); err != nil {
						m.Messages = append(m.Messages, err.Error())
					} else {
						m.Success = true
						for _, item := range removeItems {
							m.Messages = append(m.Messages, fmt.Sprintf("Clean: %s", item))
						}
					}
					ch <- m
				}
				if prune {
					if err := node.Engine.ImagesPrune(ctx); err != nil {
						log.Errorf("[RemoveImage] Prune %s pod %s node failed: %v", podname, node.Name, err)
					} else {
						log.Infof("[RemoveImage] Prune %s pod %s node", podname, node.Name)
					}
				}
			}(node)
		}
		wg.Wait()
	}()

	return ch, nil
}

// 在podname上cache这个image
// 实际上就是在所有的node上去pull一次
func (c *Calcium) doCacheImage(ctx context.Context, podname, image string) error {
	nodes, err := c.ListPodNodes(ctx, podname, false)
	if err != nil {
		return err
	}
	if len(nodes) == 0 {
		return nil
	}

	wg := sync.WaitGroup{}
	defer wg.Wait()
	for i, node := range nodes {
		// 这个函数在 create_container.go 里面
		// 同步地pull image
		if (i != 0) && (i%maxPuller == 0) {
			wg.Wait()
		}
		wg.Add(1)
		go func(node *types.Node) {
			defer wg.Done()
			pullImage(ctx, node, image)
		}(node)
	}

	return nil
}

// 清理一个pod上的全部这个image
// 对里面所有node去执行
func (c *Calcium) doCleanImage(ctx context.Context, podname, image string) error {
	nodes, err := c.ListPodNodes(ctx, podname, false)
	if err != nil {
		return err
	}
	if len(nodes) == 0 {
		return nil
	}

	wg := sync.WaitGroup{}
	defer wg.Wait()
	for i, node := range nodes {
		wg.Add(1)
		go func(node *types.Node) {
			defer wg.Done()
			if err := cleanImageOnNode(ctx, node, image, c.config.ImageCache); err != nil {
				log.Errorf("[doCleanImage] CleanImageOnNode error: %s", err)
			}
		}(node)
		if (i+1)%maxPuller == 0 {
			wg.Wait()
		}
	}
	return nil
}
