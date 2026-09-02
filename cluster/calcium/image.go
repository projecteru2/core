package calcium

import (
	"context"
	"fmt"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
)

func (c *Calcium) CacheImage(ctx context.Context, opts *types.ImageOptions) (chan *types.CacheImageMessage, error) {
	logger := log.WithFunc("calcium.CacheImage").WithField("opts", opts)
	nodes, err := c.filterNodesForImageOp(ctx, opts, logger)
	if err != nil {
		return nil, err
	}

	return perNode(c, nodes, func(node *types.Node, ch chan<- *types.CacheImageMessage) {
		for _, image := range opts.Images {
			m := &types.CacheImageMessage{
				Image:    image,
				Success:  true,
				Nodename: node.Name,
				Message:  "",
			}
			if err := pullImage(ctx, node, image); err != nil {
				logger.Error(ctx, err)
				m.Success = false
				m.Message = err.Error()
			}
			if send(ctx, ch, m) != nil {
				return
			}
		}
	}), nil
}

func (c *Calcium) RemoveImage(ctx context.Context, opts *types.ImageOptions) (chan *types.RemoveImageMessage, error) {
	logger := log.WithFunc("calcium.RemoveImage").WithField("opts", opts)
	nodes, err := c.filterNodesForImageOp(ctx, opts, logger)
	if err != nil {
		return nil, err
	}

	return perNode(c, nodes, func(node *types.Node, ch chan<- *types.RemoveImageMessage) {
		for _, image := range opts.Images {
			m := &types.RemoveImageMessage{
				Success:  false,
				Image:    image,
				Messages: []string{},
			}
			if removeItems, err := node.Engine.ImageRemove(ctx, image, false, true); err != nil {
				logger.Error(ctx, err)
				m.Messages = append(m.Messages, err.Error())
			} else {
				m.Success = true
				for _, item := range removeItems {
					m.Messages = append(m.Messages, fmt.Sprintf("Clean: %s", item))
				}
			}
			if send(ctx, ch, m) != nil {
				return
			}
		}
		if opts.Prune {
			if err := node.Engine.ImagesPrune(ctx); err != nil {
				logger.Errorf(ctx, err, "failed to prune images on node %s of pod %s", node.Name, opts.Podname)
			} else {
				logger.Infof(ctx, "pruned images on node %s of pod %s", node.Name, opts.Podname)
			}
		}
	}), nil
}

func (c *Calcium) ListImage(ctx context.Context, opts *types.ImageOptions) (chan *types.ListImageMessage, error) {
	logger := log.WithFunc("calcium.ListImage").WithField("opts", opts)
	nodes, err := c.filterNodesForImageOp(ctx, opts, logger)
	if err != nil {
		return nil, err
	}

	return perNode(c, nodes, func(node *types.Node, ch chan<- *types.ListImageMessage) {
		msg := &types.ListImageMessage{
			Images:   []*types.Image{},
			Nodename: node.Name,
			Error:    nil,
		}
		if images, err := node.Engine.ImageList(ctx, opts.Filter); err != nil {
			logger.Error(ctx, err)
			msg.Error = err
		} else {
			for _, image := range images {
				msg.Images = append(msg.Images, &types.Image{
					ID:   image.ID,
					Tags: image.Tags,
				})
			}
		}
		_ = send(ctx, ch, msg)
	}), nil
}

func (c *Calcium) filterNodesForImageOp(ctx context.Context, opts *types.ImageOptions, logger *log.Fields) ([]*types.Node, error) {
	if err := opts.Validate(); err != nil {
		logger.Error(ctx, err)
		return nil, err
	}

	nodes, err := c.filterNodes(ctx, &types.NodeFilter{Podname: opts.Podname, Includes: opts.Nodenames})
	if err != nil {
		logger.Error(ctx, err)
		return nil, err
	}

	if len(nodes) == 0 {
		logger.Error(ctx, types.ErrPodNoNodes)
		return nil, types.ErrPodNoNodes
	}
	return nodes, nil
}
