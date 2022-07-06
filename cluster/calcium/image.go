package calcium

import (
	"context"
	"fmt"
	"sync"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"

	"github.com/pkg/errors"
)

// RemoveImage remove images
func (c *Calcium) RemoveImage(ctx context.Context, opts *types.ImageOptions) (chan *types.RemoveImageMessage, error) {
	logger := log.WithField("Calcium", "RemoveImage").WithField("opts", opts)
	if err := opts.Validate(); err != nil {
		return nil, logger.ErrWithTracing(ctx, err)
	}
	opts.Normalize()

	nodes, err := c.filterNodes(ctx, types.NodeFilter{Podname: opts.Podname, Includes: opts.Nodenames})
	if err != nil {
		return nil, logger.ErrWithTracing(ctx, err)
	}

	if len(nodes) == 0 {
		return nil, logger.ErrWithTracing(ctx, errors.WithStack(types.ErrPodNoNodes))
	}

	ch := make(chan *types.RemoveImageMessage)

	utils.SentryGo(func() {
		defer close(ch)
		wg := sync.WaitGroup{}
		defer wg.Wait()
		for i, node := range nodes {
			wg.Add(1)
			utils.SentryGo(func(node *types.Node) func() {
				return func() {
					defer wg.Done()
					for _, image := range opts.Images {
						m := &types.RemoveImageMessage{
							Success:  false,
							Image:    image,
							Messages: []string{},
						}
						if removeItems, err := node.Engine.ImageRemove(ctx, image, false, true); err != nil {
							m.Messages = append(m.Messages, logger.ErrWithTracing(ctx, err).Error())
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
							logger.Errorf(ctx, "[RemoveImage] Prune %s pod %s node failed: %+v", opts.Podname, node.Name, err)
						} else {
							log.Infof(ctx, "[RemoveImage] Prune %s pod %s node", opts.Podname, node.Name)
						}
					}
				}
			}(node))
			if (i+1)%opts.Step == 0 {
				log.Info("[RemoveImage] Wait for previous cleaner done")
				wg.Wait()
			}
		}
	})

	return ch, nil
}

// CacheImage cache Image
// 在podname上cache这个image
// 实际上就是在所有的node上去pull一次
func (c *Calcium) CacheImage(ctx context.Context, opts *types.ImageOptions) (chan *types.CacheImageMessage, error) {
	logger := log.WithField("Calcium", "CacheImage").WithField("opts", opts)
	if err := opts.Validate(); err != nil {
		return nil, logger.ErrWithTracing(ctx, err)
	}
	opts.Normalize()

	nodes, err := c.filterNodes(ctx, types.NodeFilter{Podname: opts.Podname, Includes: opts.Nodenames})
	if err != nil {
		return nil, logger.ErrWithTracing(ctx, err)
	}

	if len(nodes) == 0 {
		return nil, logger.ErrWithTracing(ctx, errors.WithStack(types.ErrPodNoNodes))
	}

	ch := make(chan *types.CacheImageMessage)

	utils.SentryGo(func() {
		defer close(ch)
		wg := sync.WaitGroup{}
		defer wg.Wait()
		for i, node := range nodes {
			wg.Add(1)
			utils.SentryGo(func(node *types.Node) func() {
				return func() {
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
							m.Message = logger.ErrWithTracing(ctx, err).Error()
						}
						ch <- m
					}
				}
			}(node))
			if (i+1)%opts.Step == 0 {
				log.Info("[CacheImage] Wait for puller cleaner done")
				wg.Wait()
			}
		}
	})

	return ch, nil
}

// ListImage list Image on a pod or some nodes.
func (c *Calcium) ListImage(ctx context.Context, opts *types.ImageOptions) (chan *types.ListImageMessage, error) {
	logger := log.WithField("Calcium", "ListImage").WithField("opts", opts)
	opts.Normalize()

	nodes, err := c.filterNodes(ctx, types.NodeFilter{Podname: opts.Podname, Includes: opts.Nodenames})
	if err != nil {
		return nil, logger.ErrWithTracing(ctx, err)
	}

	if len(nodes) == 0 {
		return nil, logger.ErrWithTracing(ctx, errors.WithStack(types.ErrPodNoNodes))
	}

	ch := make(chan *types.ListImageMessage)

	utils.SentryGo(func() {
		defer close(ch)
		wg := sync.WaitGroup{}
		defer wg.Wait()
		for i, node := range nodes {
			wg.Add(1)
			utils.SentryGo(func(node *types.Node) func() {
				return func() {
					defer wg.Done()
					msg := &types.ListImageMessage{
						Images:   []*types.Image{},
						Nodename: node.Name,
						Error:    nil,
					}
					if images, err := node.Engine.ImageList(ctx, opts.Filter); err != nil {
						msg.Error = logger.ErrWithTracing(ctx, err)
					} else {
						for _, image := range images {
							msg.Images = append(msg.Images, &types.Image{
								ID:   image.ID,
								Tags: image.Tags,
							})
						}
					}
					ch <- msg
				}
			}(node))
			if (i+1)%opts.Step == 0 {
				log.Info("[ListImage] Wait for image listed")
				wg.Wait()
			}
		}
	})

	return ch, nil
}
