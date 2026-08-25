package calcium

import (
	"bytes"
	"encoding/json"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	enginemocks "github.com/projecteru2/core/engine/mocks"
	resourcemocks "github.com/projecteru2/core/resource/mocks"
	storemocks "github.com/projecteru2/core/store/mocks"
	"github.com/projecteru2/core/types"
)

const (
	base = "alpine:latest"
	repo = "https://test/repo.git"
)

func TestSelectBuildNodeRejectsAFilterItCannotNarrow(t *testing.T) {
	c := NewTestCluster()
	c.config.Build.NodeFilter = types.NodeFilter{Podname: "buildpod", Includes: []string{"n1", "n2"}}

	tests := []struct {
		name      string
		requested *types.NodeFilter
	}{
		{"another pod", &types.NodeFilter{Podname: "elsewhere"}},
		{"nodes outside the configured set", &types.NodeFilter{Includes: []string{"n9"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := c.selectBuildNode(t.Context(), &types.BuildOptions{NodeFilter: tt.requested})
			assert.ErrorIs(t, err, types.ErrInvaildNodeFilter)
		})
	}
}

func TestBuildImageOnlyNeedsTheSCMForARepo(t *testing.T) {
	buildNode := func(c *Calcium) *enginemocks.API {
		engine := &enginemocks.API{}
		node := &types.Node{NodeMeta: types.NodeMeta{Name: "test", Podname: "testpod"}, Available: true, Engine: engine}
		store := c.store.(*storemocks.Store)
		store.On("GetNodesByPod", mock.Anything, mock.Anything, mock.Anything).Return([]*types.Node{node}, nil)
		rmgr := c.rmgr.(*resourcemocks.Manager)
		rmgr.On("GetNodeResourceInfo", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil, nil, nil)
		rmgr.On("GetMostIdleNode", mock.Anything, mock.Anything).Return("test", nil)
		return engine
	}

	t.Run("a raw build never consults the scm", func(t *testing.T) {
		c := NewTestCluster()
		c.source = nil
		engine := buildNode(c)
		engine.On("BuildRefs", mock.Anything, mock.Anything).Return([]string{"t1"})
		engine.On("ImageBuild", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(io.NopCloser(bytes.NewReader(nil)), nil)
		engine.On("ImagePush", mock.Anything, mock.Anything).Return(io.NopCloser(bytes.NewReader(nil)), nil)
		engine.On("ImageRemove", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil)
		engine.On("ImageBuildCachePrune", mock.Anything, mock.Anything).Return(uint64(0), nil)

		ch, err := c.BuildImage(t.Context(), &types.BuildOptions{Name: "xx", BuildMethod: types.BuildFromRaw})
		assert.NoError(t, err)
		for msg := range ch {
			assert.Empty(t, msg.Error)
		}
	})

	t.Run("a stage with a repo still needs one", func(t *testing.T) {
		c := NewTestCluster()
		c.source = nil
		buildNode(c)

		_, err := c.BuildImage(t.Context(), &types.BuildOptions{
			Name:        "xx",
			BuildMethod: types.BuildFromSCM,
			Builds: &types.Builds{
				Stages: []string{"compile"},
				Builds: map[string]*types.Build{"compile": {Base: base, Repo: repo}},
			},
		})
		assert.ErrorIs(t, err, types.ErrNoSCMSetting)
	})
}

func TestBuild(t *testing.T) {
	c := NewTestCluster()
	ctx := t.Context()
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
	c.config.Build.NodeFilter = types.NodeFilter{Podname: "buildpod"}
	opts.NodeFilter = &types.NodeFilter{Podname: "elsewhere"}
	_, err := c.BuildImage(ctx, opts)
	assert.ErrorIs(t, err, types.ErrInvaildNodeFilter)
	opts.NodeFilter = nil
	store := c.store.(*storemocks.Store)
	store.On("GetNodesByPod", mock.Anything, mock.Anything, mock.Anything).Return(nil, types.ErrInvaildWorkloadMeta).Once()
	ch, err := c.BuildImage(ctx, opts)
	assert.Error(t, err)
	store.On("GetNodesByPod", mock.Anything, mock.Anything, mock.Anything).Return([]*types.Node{}, nil).Once()
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
	store.On("GetNodesByPod", mock.Anything, mock.Anything, mock.Anything).Return([]*types.Node{node}, nil)
	rmgr := c.rmgr.(*resourcemocks.Manager)
	rmgr.On("GetNodeResourceInfo", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil, nil, nil)
	rmgr.On("GetMostIdleNode", mock.Anything, mock.Anything).Return("", types.ErrInvaildCount).Once()
	ch, err = c.BuildImage(ctx, opts)
	assert.Error(t, err)
	rmgr.On("GetMostIdleNode", mock.Anything, mock.Anything).Return("test", nil)
	c.config.Registry.Hub = "test.com"
	c.config.Registry.Namespace = "test"

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
	buildImageRespReader := io.NopCloser(bytes.NewReader(buildImageResp))
	buildImageRespReader2 := io.NopCloser(bytes.NewReader(buildImageResp2))
	engine.On("BuildRefs", mock.Anything, mock.Anything, mock.Anything).Return([]string{"t1", "t2"})
	engine.On("BuildContent", mock.Anything, mock.Anything, mock.Anything).Return("", nil, types.ErrInvaildCount).Once()
	ch, err = c.BuildImage(ctx, opts)
	assert.Error(t, err)
	b := io.NopCloser(bytes.NewReader([]byte{}))
	engine.On("BuildContent", mock.Anything, mock.Anything, mock.Anything).Return("", b, nil)
	opts.BuildMethod = types.BuildFromRaw
	engine.On("ImageBuild", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, types.ErrNilEngine).Once()
	ch, err = c.BuildImage(ctx, opts)
	assert.Error(t, err)
	opts.BuildMethod = types.BuildFromExist
	opts.ExistID = "123"
	engine.On("ImageBuildFromExist", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("", types.ErrEngineNotImplemented).Once()
	store.On("GetWorkload", mock.Anything, mock.Anything).Return(&types.Workload{Nodename: "123"}, nil)
	store.On("GetNode", mock.Anything, mock.Anything).Return(&types.Node{Engine: engine}, nil)
	ch, err = c.BuildImage(ctx, opts)
	assert.EqualError(t, err, types.ErrEngineNotImplemented.Error())
	opts.BuildMethod = types.BuildFromUnknown
	ch, err = c.BuildImage(ctx, opts)
	assert.Error(t, err)
	engine.On("ImageBuild", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(buildImageRespReader, nil)
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
