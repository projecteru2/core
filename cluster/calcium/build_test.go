package calcium

import (
	"bytes"
	"context"
	"encoding/json"
	"io/ioutil"
	"testing"

	enginetypes "github.com/docker/docker/api/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	enginemocks "github.com/projecteru2/core/3rdmocks"
	schedulermocks "github.com/projecteru2/core/scheduler/mocks"
	sourcemocks "github.com/projecteru2/core/source/mocks"
	storemocks "github.com/projecteru2/core/store/mocks"
	"github.com/projecteru2/core/types"
)

const (
	base = "alpine:latest"
	repo = "https://test/repo.git"
)

// TODO test errors
// test no tags
func TestBuild(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()
	opts := &types.BuildOptions{
		Name: "xx",
		Builds: &types.Builds{
			Stages: []string{"compile", "build"},
			Builds: map[string]*types.Build{
				"compile": &types.Build{
					Base:      base,
					Repo:      repo,
					Version:   "version",
					Artifacts: map[string]string{"url1": "/path1", "url2": "/path2"},
					Cache:     map[string]string{"/src1": "/dst1", "/src2": "/dst2"},
					Commands:  []string{"cmd1", "cmd2"},
				},
				"build": &types.Build{
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
	// failed by buildpod not set
	ch, err := c.BuildImage(ctx, opts)
	close(ch)
	assert.Error(t, err)
	c.config.Docker.BuildPod = "test"
	// failed by ListPodNodes failed
	store := &storemocks.Store{}
	store.On("GetNodesByPod", mock.AnythingOfType("*context.emptyCtx"), mock.Anything).Return(nil, types.ErrNoBuildPod)
	c.store = store
	ch, err = c.BuildImage(ctx, opts)
	close(ch)
	assert.Error(t, err)
	// create image
	c.config.Docker.Hub = "test.com"
	c.config.Docker.Namespace = "test"

	buildImageMessage := &types.BuildImageMessage{}
	buildImageMessage.Progress = "process"
	buildImageMessage.Error = "none"
	buildImageMessage.ID = "ID"
	buildImageMessage.Status = "status"
	buildImageMessage.Stream = "stream"
	buildImageMessage.ErrorDetail.Code = 0
	buildImageResp, err := json.Marshal(buildImageMessage)
	assert.NoError(t, err)
	buildImageRespReader := enginetypes.ImageBuildResponse{Body: ioutil.NopCloser(bytes.NewReader(buildImageResp))}
	buildImageRespReader2 := ioutil.NopCloser(bytes.NewReader(buildImageResp))

	engine := &enginemocks.APIClient{}
	engine.On("ImageBuild", mock.AnythingOfType("*context.emptyCtx"), mock.Anything, mock.Anything).Return(buildImageRespReader, nil)
	engine.On("ImagePush", mock.AnythingOfType("*context.emptyCtx"), mock.Anything, mock.Anything).Return(buildImageRespReader2, nil)
	engine.On("ImageRemove", mock.AnythingOfType("*context.emptyCtx"), mock.Anything, mock.Anything).Return([]enginetypes.ImageDeleteResponseItem{}, nil)
	r := &enginetypes.BuildCachePruneReport{SpaceReclaimed: 1024}
	engine.On("BuildCachePrune", mock.AnythingOfType("*context.emptyCtx"), mock.Anything).Return(r, nil)
	node := &types.Node{
		Name:      "test",
		Available: true,
		Engine:    engine,
	}
	scheduler := &schedulermocks.Scheduler{}
	scheduler.On("MaxCPUIdleNode", mock.AnythingOfType("[]*types.Node")).Return(node)
	store = &storemocks.Store{}
	store.On("GetNodesByPod", mock.AnythingOfType("*context.emptyCtx"), mock.Anything).Return([]*types.Node{node}, nil)
	source := &sourcemocks.Source{}
	source.On("SourceCode", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	source.On("Security", mock.Anything).Return(nil)
	source.On("Artifact", mock.Anything, mock.Anything).Return(nil)

	c.scheduler = scheduler
	c.store = store
	c.source = source

	ch, err = c.BuildImage(ctx, opts)
	for range ch {
		assert.NoError(t, err)
	}
}
