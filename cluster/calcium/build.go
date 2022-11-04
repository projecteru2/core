package calcium

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/cockroachdb/errors"
	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

// BuildImage will build image
func (c *Calcium) BuildImage(ctx context.Context, opts *types.BuildOptions) (ch chan *types.BuildImageMessage, err error) {
	logger := log.WithFunc("calcium.BuildImage").WithField("opts", opts)
	// Disable build API if scm not set
	if c.source == nil {
		return nil, types.ErrNoSCMSetting
	}
	// select nodes
	node, err := c.selectBuildNode(ctx)
	if err != nil {
		logger.Error(ctx, err)
		return nil, err
	}

	logger.Infof(ctx, "Building image at pod %s node %s", node.Podname, node.Name)

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
	ch, err = c.pushImageAndClean(ctx, resp, node, refs)
	logger.Error(ctx, err)
	return ch, err
}

func (c *Calcium) selectBuildNode(ctx context.Context) (*types.Node, error) {
	// get pod from config
	// TODO can choose multiple pod here for other engine support
	if c.config.Docker.BuildPod == "" {
		return nil, types.ErrNoBuildPod
	}

	// get nodes
	nodes, err := c.store.GetNodesByPod(ctx, &types.NodeFilter{Podname: c.config.Docker.BuildPod})
	if err != nil {
		return nil, err
	}

	if len(nodes) == 0 {
		return nil, types.ErrInsufficientCapacity
	}
	// get idle max node
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
	buildContentOpts := &enginetypes.BuildContentOptions{
		User:   opts.User,
		UID:    opts.UID,
		Builds: opts.Builds,
	}
	path, content, err := node.Engine.BuildContent(ctx, c.source, buildContentOpts)
	defer os.RemoveAll(path)
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

func (c *Calcium) pushImageAndClean(ctx context.Context, resp io.ReadCloser, node *types.Node, tags []string) (chan *types.BuildImageMessage, error) { //nolint:unparam
	logger := log.WithFunc("calcium.pushImageAndClean").WithField("node", node).WithField("tags", tags)
	logger.Infof(ctx, "Pushing image at pod %s node %s", node.Podname, node.Name)
	return c.withImageBuiltChannel(func(ch chan *types.BuildImageMessage) {
		defer resp.Close()
		decoder := json.NewDecoder(resp)
		lastMessage := &types.BuildImageMessage{}
		for {
			message := &types.BuildImageMessage{}
			if err := decoder.Decode(message); err != nil {
				if err == io.EOF {
					break
				}
				if err == context.Canceled || err == context.DeadlineExceeded {
					logger.Error(ctx, err, "context timeout")
					lastMessage.ErrorDetail.Code = -1
					lastMessage.ErrorDetail.Message = err.Error()
					lastMessage.Error = err.Error()
					break
				}
				malformed, _ := io.ReadAll(decoder.Buffered()) // TODO err check
				logger.Errorf(ctx, err, "[BuildImage] Decode build image message failed, buffered: %+v", malformed)
				return
			}
			ch <- message
			lastMessage = message
		}

		if lastMessage.Error != "" {
			logger.Errorf(ctx, errors.New(lastMessage.Error), "[BuildImage] Build image failed %+v", lastMessage.ErrorDetail.Message)
			return
		}

		// push and clean
		for i := range tags {
			tag := tags[i]
			logger.Infof(ctx, "Push image %s", tag)
			rc, err := node.Engine.ImagePush(ctx, tag)
			if err != nil {
				logger.Error(ctx, err)
				ch <- &types.BuildImageMessage{Error: err.Error()}
				continue
			}

			for message := range c.processBuildImageStream(ctx, rc) {
				ch <- message
			}

			ch <- &types.BuildImageMessage{Stream: fmt.Sprintf("finished %s\n", tag), Status: "finished", Progress: tag}
		}
		// 无论如何都删掉build机器的
		// 事实上他不会跟cached pod一样
		// 一样就砍死
		_ = c.pool.Invoke(func() {
			cleanupNodeImages(ctx, node, tags, c.config.GlobalTimeout)
		})
	}), nil

}

func (c *Calcium) getWorkloadNode(ctx context.Context, id string) (*types.Node, error) {
	w, err := c.store.GetWorkload(ctx, id)
	if err != nil {
		return nil, err
	}
	node, err := c.store.GetNode(ctx, w.Nodename)
	return node, err
}

func (c *Calcium) withImageBuiltChannel(f func(chan *types.BuildImageMessage)) chan *types.BuildImageMessage {
	ch := make(chan *types.BuildImageMessage)
	_ = c.pool.Invoke(func() {
		defer close(ch)
		f(ch)
	})
	return ch
}

func cleanupNodeImages(ctx context.Context, node *types.Node, ids []string, ttl time.Duration) {
	logger := log.WithFunc("calcium.cleanupNodeImages").WithField("node", node).WithField("ids", ids).WithField("ttl", ttl)
	ctx, cancel := context.WithTimeout(utils.InheritTracingInfo(ctx, context.TODO()), ttl)
	defer cancel()
	for _, id := range ids {
		if _, err := node.Engine.ImageRemove(ctx, id, false, true); err != nil {
			logger.Error(ctx, err, "Remove image error")
		}
	}
	if spaceReclaimed, err := node.Engine.ImageBuildCachePrune(ctx, true); err != nil {
		logger.Error(ctx, err, "Remove build image cache error")
	} else {
		logger.Infof(ctx, "Clean cached image and release space %d", spaceReclaimed)
	}
}

func toBuildRefOptions(opts *types.BuildOptions) *enginetypes.BuildRefOptions {
	return &enginetypes.BuildRefOptions{
		Name: opts.Name,
		Tags: opts.Tags,
		User: opts.User,
	}
}
