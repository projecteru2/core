package calcium

import (
	"bytes"
	"context"
	"encoding/json"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	enginemocks "github.com/projecteru2/core/engine/mocks"
	schedulermocks "github.com/projecteru2/core/scheduler/mocks"
	storemocks "github.com/projecteru2/core/store/mocks"
	"github.com/projecteru2/core/types"
)

const (
	base = "alpine:latest"
	repo = "https://test/repo.git"
)

// test no tags
func TestBuild(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()
	opts := &types.BuildOptions{
		Name:        "xx",
		BuildMethod: types.BuildFromSCM,
		Builds: &types.Builds{
			Stages: []string{"compile", "build"},
			Builds: map[string]*types.Build{
				"compile": {
					Base:      base,
					Repo:      repo,
					Version:   "version",
					Artifacts: map[string]string{"url1": "/path1", "url2": "/path2"},
					Cache:     map[string]string{"/src1": "/dst1", "/src2": "/dst2"},
					Commands:  []string{"cmd1", "cmd2"},
				},
				"build": {
					Base:     base,
					Commands: []string{"cmd1", "cmd2"},
					Args:     map[string]string{"args1": "a", "args2": "b"},
					Envs:     map[string]string{"envs1": "a", "envs2": "b"},
					Labels:   map[string]string{"labels": "a", "label2": "b"},
					Dir:      "/tmp",
				},
			},
		},
		UID:  1234,
		User: "test",
		Tags: []string{"tag1", "tag2"},
	}
	// failed by no source
	c.source = nil
	_, err := c.BuildImage(ctx, opts)
	assert.Error(t, err)
	// failed by buildpod not set
	c = NewTestCluster()
	_, err = c.BuildImage(ctx, opts)
	assert.Error(t, err)
	c.config.Docker.BuildPod = "test"
	// failed by ListPodNodes failed
	store := &storemocks.Store{}
	store.On("GetNodesByPod", mock.AnythingOfType("*context.emptyCtx"), mock.Anything, mock.Anything, mock.Anything).Return(nil, types.ErrBadMeta).Once()
	c.store = store
	ch, err := c.BuildImage(ctx, opts)
	assert.Error(t, err)
	// failed by no nodes
	store.On("GetNodesByPod", mock.AnythingOfType("*context.emptyCtx"), mock.Anything, mock.Anything, mock.Anything).Return([]*types.Node{}, nil).Once()
	ch, err = c.BuildImage(ctx, opts)
	assert.Error(t, err)
	engine := &enginemocks.API{}
	node := &types.Node{
		NodeMeta: types.NodeMeta{
			Name:    "test",
			Podname: "testpod",
		},
		Available: true,
		Engine:    engine,
	}
	store.On("GetNodesByPod", mock.AnythingOfType("*context.emptyCtx"), mock.Anything, mock.Anything, mock.Anything).Return([]*types.Node{node}, nil)
	scheduler := &schedulermocks.Scheduler{}
	c.scheduler = scheduler
	// failed by MaxIdleNode
	scheduler.On("MaxIdleNode", mock.AnythingOfType("[]*types.Node")).Return(nil, types.ErrBadMeta).Once()
	ch, err = c.BuildImage(ctx, opts)
	assert.Error(t, err)
	scheduler.On("MaxIdleNode", mock.AnythingOfType("[]*types.Node")).Return(node, nil)
	// create image
	c.config.Docker.Hub = "test.com"
	c.config.Docker.Namespace = "test"

	buildImageMessage := &types.BuildImageMessage{}
	buildImageMessage.Progress = "process"
	buildImageMessage.Error = ""
	buildImageMessage.ID = "ID1234"
	buildImageMessage.Status = "status"
	buildImageMessage.Stream = "stream"
	buildImageMessage.ErrorDetail.Code = 0
	buildImageResp, err := json.Marshal(buildImageMessage)
	assert.NoError(t, err)
	buildImageResp2, err := json.Marshal(buildImageMessage)
	assert.NoError(t, err)
	buildImageRespReader := ioutil.NopCloser(bytes.NewReader(buildImageResp))
	buildImageRespReader2 := ioutil.NopCloser(bytes.NewReader(buildImageResp2))
	engine.On("BuildRefs", mock.Anything, mock.Anything, mock.Anything).Return([]string{"t1", "t2"})
	// failed by build context
	engine.On("BuildContent", mock.Anything, mock.Anything, mock.Anything).Return("", nil, types.ErrBadCount).Once()
	ch, err = c.BuildImage(ctx, opts)
	assert.Error(t, err)
	b := ioutil.NopCloser(bytes.NewReader([]byte{}))
	engine.On("BuildContent", mock.Anything, mock.Anything, mock.Anything).Return("", b, nil)
	// failed by ImageBuild
	opts.BuildMethod = types.BuildFromRaw
	engine.On("ImageBuild", mock.Anything, mock.Anything, mock.Anything).Return(nil, types.ErrNilEngine).Once()
	ch, err = c.BuildImage(ctx, opts)
	assert.Error(t, err)
	// build from exist not implemented
	opts.BuildMethod = types.BuildFromExist
	engine.On("ImageBuildFromExist", mock.Anything, mock.Anything, mock.Anything).Return("", types.ErrEngineNotImplemented).Once()
	store.On("GetWorkload", mock.Anything, mock.Anything).Return(&types.Workload{}, nil)
	store.On("GetNode", mock.Anything, mock.Anything).Return(&types.Node{Engine: engine}, nil)
	ch, err = c.BuildImage(ctx, opts)
	assert.NoError(t, err)
	// unknown build method
	opts.BuildMethod = types.BuildFromUnknown
	ch, err = c.BuildImage(ctx, opts)
	assert.Error(t, err)
	// correct
	engine.On("ImageBuild", mock.Anything, mock.Anything, mock.Anything).Return(buildImageRespReader, nil)
	engine.On("ImagePush", mock.Anything, mock.Anything).Return(buildImageRespReader2, nil)
	engine.On("ImageRemove", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return([]string{}, nil)
	engine.On("ImageBuildCachePrune", mock.Anything, mock.Anything).Return(uint64(1024), nil)
	engine.On("BuildContent", mock.Anything, mock.Anything, mock.Anything).Return("", nil, nil)
	opts.BuildMethod = types.BuildFromSCM
	ch, err = c.BuildImage(ctx, opts)
	if assert.NoError(t, err) {
		for range ch {
			assert.NoError(t, err)
		}
	}
}
