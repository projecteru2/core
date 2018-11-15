package calcium

import (
	"context"
	"fmt"
	"sync"

	enginetypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
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

	opts := enginetypes.ImageRemoveOptions{
		Force:         false,
		PruneChildren: true,
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
					if removeItems, err := node.Engine.ImageRemove(ctx, image, opts); err != nil {
						m.Messages = append(m.Messages, err.Error())
					} else {
						m.Success = true
						for _, item := range removeItems {
							if item.Untagged != "" {
								m.Messages = append(m.Messages, fmt.Sprintf("Untagged: %s", item.Untagged))
							}
							if item.Deleted != "" {
								m.Messages = append(m.Messages, fmt.Sprintf("Deleted: %s", item.Deleted))
							}
						}
					}
					ch <- m
				}
				if prune {
					_, err := node.Engine.ImagesPrune(ctx, filters.NewArgs())
					if err != nil {
						log.Errorf("[CleanPod] Prune %s pod %s node failed: %v", podname, node.Name, err)
					} else {
						log.Infof("[CleanPod] Prune %s pod %s node", podname, node.Name)
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
func (c *Calcium) cacheImage(ctx context.Context, podname, image string) error {
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
			auth, err := makeEncodedAuthConfigFromRemote(c.config.Docker.AuthConfigs, image)
			if err != nil {
				log.Errorf("[cacheImage] Cache image failed %v", err)
				return
			}
			pullImage(ctx, node, image, auth)
		}(node)
	}

	return nil
}

// 清理一个pod上的全部这个image
// 对里面所有node去执行
func (c *Calcium) cleanImage(ctx context.Context, podname, image string) error {
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
				log.Errorf("[cleanImage] CleanImageOnNode error: %s", err)
			}
		}(node)
		if (i+1)%maxPuller == 0 {
			wg.Wait()
		}
	}
	return nil
}
