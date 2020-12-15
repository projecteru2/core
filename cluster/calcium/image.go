package calcium

import (
	"context"
	"fmt"
	"sync"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
)

// RemoveImage remove images
func (c *Calcium) RemoveImage(ctx context.Context, opts *types.ImageOptions) (chan *types.RemoveImageMessage, error) {
	if err := opts.Validate(); err != nil {
		return nil, err
	}
	opts.Normalize()

	nodes, err := c.getNodes(ctx, opts.Podname, opts.Nodenames, nil, false)
	if err != nil {
		return nil, err
	}

	if len(nodes) == 0 {
		return nil, types.ErrPodNoNodes
	}

	ch := make(chan *types.RemoveImageMessage)

	go func() {
		defer close(ch)
		wg := sync.WaitGroup{}
		defer wg.Wait()
		for i, node := range nodes {
			wg.Add(1)
			go func(node *types.Node) {
				defer wg.Done()
				for _, image := range opts.Images {
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
				if opts.Prune {
					if err := node.Engine.ImagesPrune(ctx); err != nil {
						log.Errorf("[RemoveImage] Prune %s pod %s node failed: %v", opts.Podname, node.Name, err)
					} else {
						log.Infof("[RemoveImage] Prune %s pod %s node", opts.Podname, node.Name)
					}
				}
			}(node)
			if (i+1)%opts.Step == 0 {
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
func (c *Calcium) CacheImage(ctx context.Context, opts *types.ImageOptions) (chan *types.CacheImageMessage, error) {
	if err := opts.Validate(); err != nil {
		return nil, err
	}
	opts.Normalize()

	nodes, err := c.getNodes(ctx, opts.Podname, opts.Nodenames, nil, false)
	if err != nil {
		return nil, err
	}

	if len(nodes) == 0 {
		return nil, types.ErrPodNoNodes
	}

	ch := make(chan *types.CacheImageMessage)

	go func() {
		defer close(ch)
		wg := sync.WaitGroup{}
		defer wg.Wait()
		for i, node := range nodes {
			wg.Add(1)
			go func(node *types.Node) {
				defer wg.Done()
				for _, image := range opts.Images {
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
			if (i+1)%opts.Step == 0 {
				log.Info("[CacheImage] Wait for puller cleaner done")
				wg.Wait()
			}
		}
	}()

	return ch, nil
}
