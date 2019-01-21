package calcium

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/types"
	log "github.com/sirupsen/logrus"
)

// BuildImage will build image
func (c *Calcium) BuildImage(ctx context.Context, opts *enginetypes.BuildOptions) (chan *types.BuildImageMessage, error) {
	// select nodes
	node, err := c.selectBuildNode(ctx)
	if err != nil {
		return nil, err
	}
	log.Infof("[BuildImage] Building image at pod %s node %s", node.Podname, node.Name)
	// get refs
	refs := node.Engine.BuildRefs(ctx, opts.Name, opts.Tags)
	// support raw build
	buildContent := opts.Tar
	if opts.Builds != nil {
		buildContent, err = node.Engine.BuildContent(ctx, c.source, opts)
		if err != nil {
			return nil, err
		}
	}

	return c.doBuildImage(ctx, buildContent, node, refs)
}

func (c *Calcium) selectBuildNode(ctx context.Context) (*types.Node, error) {
	// get pod from config
	// TODO VM BRANCH conside vm build machines.
	if c.config.Docker.BuildPod == "" {
		return nil, types.ErrNoBuildPod
	}

	// get node by scheduler
	nodes, err := c.ListPodNodes(ctx, c.config.Docker.BuildPod, false)
	if err != nil {
		return nil, err
	}
	if len(nodes) == 0 {
		return nil, types.ErrInsufficientNodes
	}
	// get idle max node
	return c.scheduler.MaxIdleNode(nodes)
}

func (c *Calcium) doBuildImage(ctx context.Context, buildContext io.ReadCloser, node *types.Node, tags []string) (chan *types.BuildImageMessage, error) {
	ch := make(chan *types.BuildImageMessage)
	// must be put here because of that `defer os.RemoveAll(buildDir)`

	resp, err := node.Engine.ImageBuild(ctx, buildContext, tags)
	if err != nil {
		close(ch)
		return ch, err
	}

	go func() {
		defer resp.Close()
		defer close(ch)
		decoder := json.NewDecoder(resp)
		var lastMessage *types.BuildImageMessage
		for {
			message := &types.BuildImageMessage{}
			err := decoder.Decode(message)
			if err != nil {
				if err == io.EOF {
					break
				}
				if err == context.Canceled || err == context.DeadlineExceeded {
					lastMessage.ErrorDetail.Code = -1
					lastMessage.Error = err.Error()
					break
				}
				malformed := []byte{}
				_, _ = decoder.Buffered().Read(malformed)
				log.Errorf("[BuildImage] Decode build image message failed %v, buffered: %v", err, malformed)
				return
			}
			ch <- message
			lastMessage = message
		}

		if lastMessage.Error != "" {
			log.Errorf("[BuildImage] Build image failed %v", lastMessage.ErrorDetail.Message)
			return
		}

		// push and clean
		for i := range tags {
			tag := tags[i]
			log.Infof("[BuildImage] Push image %s", tag)
			rc, err := node.Engine.ImagePush(ctx, tag)
			if err != nil {
				ch <- makeErrorBuildImageMessage(err)
				continue
			}
			defer rc.Close()

			decoder2 := json.NewDecoder(rc)
			for {
				message := &types.BuildImageMessage{}
				err := decoder2.Decode(message)
				if err != nil {
					if err == io.EOF {
						break
					}
					malformed := []byte{}
					_, _ = decoder2.Buffered().Read(malformed)
					log.Errorf("[BuildImage] Decode push image message failed %v, buffered: %v", err, malformed)
					break
				}
				ch <- message
			}

			// 无论如何都删掉build机器的
			// 事实上他不会跟cached pod一样
			// 一样就砍死
			go func(tag string) {
				//CONTEXT 这里的不应该受到 client 的影响
				ctx := context.Background()
				_, err := node.Engine.ImageRemove(ctx, tag, false, true)
				if err != nil {
					log.Errorf("[BuildImage] Remove image error: %s", err)
				}
				spaceReclaimed, err := node.Engine.ImageBuildCachePrune(ctx, true)
				if err != nil {
					log.Errorf("[BuildImage] Remove build image cache error: %s", err)
				}
				log.Infof("[BuildImage] Clean cached image and release space %d", spaceReclaimed)
			}(tag)

			ch <- &types.BuildImageMessage{Stream: fmt.Sprintf("finished %s\n", tag), Status: "finished", Progress: tag}
		}
	}()

	return ch, nil
}
