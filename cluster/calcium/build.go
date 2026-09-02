package calcium

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"slices"
	"time"

	"github.com/cockroachdb/errors"

	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

func (c *Calcium) BuildImage(ctx context.Context, opts *types.BuildOptions) (chan *types.BuildImageMessage, error) {
	logger := log.WithFunc("calcium.BuildImage").WithField("opts", opts)
	node, err := c.selectBuildNode(ctx, opts)
	if err != nil {
		logger.Error(ctx, err)
		return nil, err
	}

	logger.Infof(ctx, "building image at pod %s node %s", node.Podname, node.Name)

	var (
		refs []string
		resp io.ReadCloser
	)
	switch opts.BuildMethod {
	case types.BuildFromSCM:
		refs, resp, err = c.buildFromSCM(ctx, node, opts)
	case types.BuildFromRaw:
		refs, resp, err = c.buildFromContent(ctx, node, opts)
	case types.BuildFromExist:
		refs, node, resp, err = c.buildFromExist(ctx, opts)
	default:
		return nil, types.ErrInvaildBuildType
	}
	if err != nil {
		logger.Error(ctx, err)
		return nil, err
	}
	return c.pushImageAndClean(ctx, resp, node, refs), nil
}

func (c *Calcium) selectBuildNode(ctx context.Context, opts *types.BuildOptions) (*types.Node, error) {
	filter, err := c.config.Build.NodeFilter.Narrow(opts.NodeFilter)
	if err != nil {
		return nil, err
	}

	nodes, err := c.store.GetNodesByPod(ctx, filter, false)
	if err != nil {
		return nil, err
	}
	nodes = slices.DeleteFunc(nodes, func(n *types.Node) bool {
		return len(filter.Includes) > 0 && !slices.Contains(filter.Includes, n.Name) ||
			slices.Contains(filter.Excludes, n.Name)
	})

	if len(nodes) == 0 {
		return nil, types.ErrInsufficientCapacity
	}
	return c.getMostIdleNode(ctx, nodes)
}

func (c *Calcium) getMostIdleNode(ctx context.Context, nodes []*types.Node) (*types.Node, error) {
	nodenames := []string{}
	nodeMap := map[string]*types.Node{}
	for _, node := range nodes {
		nodenames = append(nodenames, node.Name)
		nodeMap[node.Name] = node
	}

	mostIdleNode, err := c.rmgr.GetMostIdleNode(ctx, nodenames)
	if err != nil {
		return nil, err
	}
	return nodeMap[mostIdleNode], nil
}

func (c *Calcium) buildFromSCM(ctx context.Context, node *types.Node, opts *types.BuildOptions) ([]string, io.ReadCloser, error) {
	if c.source == nil && needsSCM(opts.Builds) {
		return nil, nil, types.ErrNoSCMSetting
	}
	buildContentOpts := &enginetypes.BuildContentOptions{
		User:   opts.User,
		UID:    opts.UID,
		Builds: opts.Builds,
	}
	path, content, err := node.Engine.BuildContent(ctx, c.source, buildContentOpts)
	defer func() {
		_ = os.RemoveAll(path)
	}()
	if err != nil {
		return nil, nil, err
	}
	opts.Tar = content
	return c.buildFromContent(ctx, node, opts)
}

func (c *Calcium) buildFromContent(ctx context.Context, node *types.Node, opts *types.BuildOptions) ([]string, io.ReadCloser, error) {
	refs := node.Engine.BuildRefs(ctx, toBuildRefOptions(opts))
	resp, err := node.Engine.ImageBuild(ctx, opts.Tar, refs, opts.Platform)
	return refs, resp, err
}

func (c *Calcium) buildFromExist(ctx context.Context, opts *types.BuildOptions) (refs []string, node *types.Node, resp io.ReadCloser, err error) {
	if node, err = c.getWorkloadNode(ctx, opts.ExistID); err != nil {
		return nil, nil, nil, err
	}

	refs = node.Engine.BuildRefs(ctx, toBuildRefOptions(opts))
	imgID, err := node.Engine.ImageBuildFromExist(ctx, opts.ExistID, refs, opts.User)
	if err != nil {
		return nil, nil, nil, err
	}

	buildMsg, err := json.Marshal(types.BuildImageMessage{ID: imgID})
	if err != nil {
		return nil, nil, nil, err
	}

	return refs, node, io.NopCloser(bytes.NewReader(buildMsg)), nil
}

func (c *Calcium) pushImageAndClean(ctx context.Context, resp io.ReadCloser, node *types.Node, tags []string) chan *types.BuildImageMessage {
	logger := log.WithFunc("calcium.pushImageAndClean").WithField("node", node).WithField("tags", tags)
	logger.Infof(ctx, "pushing image at pod %s node %s", node.Podname, node.Name)
	return c.withImageBuiltChannel(func(ch chan *types.BuildImageMessage) {
		defer func() {
			_ = resp.Close()
		}()
		decoder := json.NewDecoder(resp)
		lastMessage := &types.BuildImageMessage{}
		for {
			message := &types.BuildImageMessage{}
			if err := decoder.Decode(message); err != nil {
				if errors.Is(err, io.EOF) {
					break
				}
				malformed, _ := io.ReadAll(decoder.Buffered())
				logger.Errorf(ctx, err, "decode build image message failed, buffered: %s", malformed)
				return
			}
			if send(ctx, ch, message) != nil {
				return
			}
			lastMessage = message
		}

		if lastMessage.Error != "" {
			logger.Errorf(ctx, errors.New(lastMessage.Error), "build image failed: %s", lastMessage.ErrorDetail.Message)
			return
		}

		for _, tag := range tags {
			logger.Infof(ctx, "push image %s", tag)
			rc, err := node.Engine.ImagePush(ctx, tag)
			if err != nil {
				logger.Error(ctx, err)
				if send(ctx, ch, &types.BuildImageMessage{Error: err.Error()}) != nil {
					return
				}
				continue
			}

			for message := range c.processBuildImageStream(ctx, rc) {
				if send(ctx, ch, message) != nil {
					return
				}
			}

			if send(ctx, ch, &types.BuildImageMessage{Stream: fmt.Sprintf("finished %s\n", tag), Status: "finished", Progress: tag}) != nil {
				return
			}
		}
		_ = c.pool.Invoke(func() {
			cleanupNodeImages(ctx, node, tags, c.config.GlobalTimeout)
		})
	})
}

func (c *Calcium) getWorkloadNode(ctx context.Context, ID string) (*types.Node, error) {
	w, err := c.store.GetWorkload(ctx, ID)
	if err != nil {
		return nil, err
	}
	node, err := c.store.GetNode(ctx, w.Nodename)
	return node, err
}

func (c *Calcium) withImageBuiltChannel(f func(chan *types.BuildImageMessage)) chan *types.BuildImageMessage {
	ch := make(chan *types.BuildImageMessage)
	utils.SentryGo(func() {
		defer close(ch)
		f(ch)
	})
	return ch
}

func cleanupNodeImages(ctx context.Context, node *types.Node, IDs []string, ttl time.Duration) {
	logger := log.WithFunc("calcium.cleanupNodeImages").WithField("node", node).WithField("IDs", IDs).WithField("ttl", ttl)
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), ttl)
	defer cancel()
	for _, ID := range IDs {
		if _, err := node.Engine.ImageRemove(ctx, ID, false, true); err != nil {
			logger.Error(ctx, err, "remove image")
		}
	}
	if spaceReclaimed, err := node.Engine.ImageBuildCachePrune(ctx, true); err != nil {
		logger.Error(ctx, err, "remove build image cache")
	} else {
		logger.Infof(ctx, "clean cached image and release space %d", spaceReclaimed)
	}
}

func needsSCM(builds *enginetypes.Builds) bool {
	if builds == nil {
		return false
	}
	return slices.ContainsFunc(builds.Stages, func(stage string) bool {
		build, ok := builds.Builds[stage]
		return ok && build.Repo != ""
	})
}

func toBuildRefOptions(opts *types.BuildOptions) *enginetypes.BuildRefOptions {
	return &enginetypes.BuildRefOptions{
		Name: opts.Name,
		Tags: opts.Tags,
		User: opts.User,
	}
}
