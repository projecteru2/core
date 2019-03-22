package calcium

import (
	"context"
	"fmt"
	"sync"

	"github.com/projecteru2/core/types"
	log "github.com/sirupsen/logrus"
)

// RemoveImage remove images
func (c *Calcium) RemoveImage(ctx context.Context, podname, nodename string, images []string, step int, prune bool) (chan *types.RemoveImageMessage, error) {
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

	if len(nodes) == 0 {
		return nil, types.ErrPodNoNodes
	}

	go func() {
		defer close(ch)
		wg := sync.WaitGroup{}
		defer wg.Wait()
		for i, node := range nodes {
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
			if (i+1)%step == 0 {
				log.Info("[RemoveImage] Wait for previous cleaner done")
				wg.Wait()
			}
		}
	}()

	return ch, nil
}

// CacheImage cache Image
// 在podname上cache这个image
// 实际上就是在所有的node上去pull一次
func (c *Calcium) CacheImage(ctx context.Context, podname, nodename string, images []string, step int) (chan *types.CacheImageMessage, error) {
	ch := make(chan *types.CacheImageMessage)

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

	if len(nodes) == 0 {
		return nil, types.ErrPodNoNodes
	}

	go func() {
		defer close(ch)
		wg := sync.WaitGroup{}
		defer wg.Wait()
		for i, node := range nodes {
			wg.Add(1)
			go func(node *types.Node) {
				defer wg.Done()
				for _, image := range images {
					m := &types.CacheImageMessage{
						Image:    image,
						Success:  true,
						Nodename: node.Name,
						Message:  "",
					}
					if err := pullImage(ctx, node, image); err != nil {
						m.Success = false
						m.Message = err.Error()
					}
					ch <- m
				}
			}(node)
			if (i+1)%step == 0 {
				log.Info("[CacheImage] Wait for puller cleaner done")
				wg.Wait()
			}
		}
	}()

	return ch, nil
}
