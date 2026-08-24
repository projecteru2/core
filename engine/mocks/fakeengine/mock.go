package fakeengine

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/docker/go-units"
	"github.com/sanity-io/litter"
	mock "github.com/stretchr/testify/mock"

	"github.com/projecteru2/core/engine"
	enginemocks "github.com/projecteru2/core/engine/mocks"
	enginetypes "github.com/projecteru2/core/engine/types"
	resourcetypes "github.com/projecteru2/core/resource/types"
	coretypes "github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

// PrefixKey is the endpoint prefix for the in-memory mock engine.
const PrefixKey = "mock://"

type writeCloser struct {
	*bufio.Writer
}

func (wc *writeCloser) Close() error {
	return nil
}

// MakeClient builds an engine.API whose calls return canned data.
func MakeClient(_ context.Context, _ coretypes.Config, nodename, endpoint, ca, cert, key string) (engine.API, error) {
	e := &enginemocks.API{}
	params := &enginetypes.Params{
		Nodename: nodename,
		Endpoint: endpoint,
		CA:       ca,
		Cert:     cert,
		Key:      key,
	}
	e.On("Info", mock.Anything).Return(&enginetypes.Info{NCPU: 100, MemTotal: units.GiB * 100, StorageTotal: units.GiB * 100}, nil)
	e.On("Ping", mock.Anything).Return(nil)
	e.On("GetParams").Return(params)
	e.On("CloseConn").Return(nil)
	e.On("Execute", mock.Anything, mock.Anything, mock.Anything).Return(
		func(context.Context, string, *enginetypes.ExecConfig) string {
			return utils.RandomString(64)
		},
		func(context.Context, string, *enginetypes.ExecConfig) io.ReadCloser {
			return io.NopCloser(bytes.NewBufferString(utils.RandomString(128)))
		},
		func(context.Context, string, *enginetypes.ExecConfig) io.ReadCloser {
			return io.NopCloser(bytes.NewBufferString(utils.RandomString(128)))
		},
		func(context.Context, string, *enginetypes.ExecConfig) io.WriteCloser {
			return &writeCloser{bufio.NewWriter(bytes.NewBuffer([]byte{}))}
		},
		nil,
	)
	e.On("ExecResize", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	e.On("ExecExitCode", mock.Anything, mock.Anything, mock.Anything).Return(0, nil)
	e.On("NetworkConnect", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return([]string{}, nil)
	e.On("NetworkDisconnect", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	e.On("NetworkList", mock.Anything, mock.Anything).Return([]*enginetypes.Network{{
		Name: "mock-network", Subnets: []string{"1.1.1.1/8", "2.2.2.2/8"},
	}}, nil)
	e.On("ImageList", mock.Anything, mock.Anything).Return(
		[]*enginetypes.Image{{ID: "mock-image", Tags: []string{"latest"}}}, nil,
	)
	e.On("ImageRemove", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(
		[]string{"mock-image1", "mock-image2"}, nil,
	)
	e.On("ImagesPrune", mock.Anything).Return(nil)
	pullImageData := io.NopCloser(bytes.NewBufferString("pull image layer1 ...\npull image layer2...\n"))
	e.On("ImagePull", mock.Anything, mock.Anything, mock.Anything).Return(pullImageData, nil)
	pushImageData := io.NopCloser(bytes.NewBufferString("{\"stream\":\"push something...\"}\n"))
	e.On("ImagePush", mock.Anything, mock.Anything).Return(pushImageData, nil)
	buildImageData := io.NopCloser(bytes.NewBufferString("{\"stream\":\"build something...\"}\n"))
	e.On("ImageBuild", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(buildImageData, nil)
	e.On("ImageBuildCachePrune", mock.Anything, mock.Anything).Return(uint64(0), nil)
	imageDigest := utils.RandomString(64)
	e.On("ImageLocalDigests", mock.Anything, mock.Anything).Return([]string{imageDigest}, nil)
	e.On("ImageRemoteDigest", mock.Anything, mock.Anything).Return(imageDigest, nil)
	e.On("ImageBuildFromExist", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("ImageBuildFromExist", nil)
	e.On("BuildRefs", mock.Anything, mock.Anything).Return([]string{"ref1", "ref2"})
	buildContent := io.NopCloser(bytes.NewBufferString("this is content"))
	e.On("BuildContent", mock.Anything, mock.Anything, mock.Anything).Return("BuildContent", buildContent, nil)
	var ID string
	e.On("VirtualizationCreate", mock.Anything, mock.Anything).Return(func(_ context.Context, opts *enginetypes.VirtualizationCreateOptions) *enginetypes.VirtualizationCreated {
		resourceOpts := &engine.VirtualizationResource{}
		_ = resourceOpts.Decode(opts.EngineParams)
		litter.Dump(resourceOpts)
		ID = utils.RandomString(64)
		return &enginetypes.VirtualizationCreated{ID: ID, Name: "mock-test-cvm" + utils.RandomString(6)}
	}, nil)
	e.On("VirtualizationCopyTo", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	e.On("VirtualizationCopyChunkTo", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	e.On("VirtualizationStart", mock.Anything, mock.Anything).Return(nil)
	e.On("VirtualizationStop", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	e.On("VirtualizationRemove", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	e.On("VirtualizationSuspend", mock.Anything, mock.Anything).Return(nil)
	e.On("VirtualizationResume", mock.Anything, mock.Anything).Return(nil)
	vcJSON := &enginetypes.VirtualizationInfo{ID: ID, Image: "mock-image", Running: true, Networks: map[string]string{"mock-network": "1.1.1.1"}}
	e.On("VirtualizationInspect", mock.Anything, mock.Anything).Return(vcJSON, nil)
	logs := io.NopCloser(bytes.NewBufferString("logs1...\nlogs2...\n"))
	e.On("VirtualizationLogs", mock.Anything, mock.Anything).Return(logs, logs, nil)
	attachData := io.NopCloser(bytes.NewBufferString("logs1...\nlogs2...\n"))
	writeBuffer := &writeCloser{bufio.NewWriter(bytes.NewBuffer([]byte{}))}
	e.On("VirtualizationAttach", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(attachData, attachData, writeBuffer, nil)
	e.On("VirtualizationResize", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	e.On("VirtualizationWait", mock.Anything, mock.Anything, mock.Anything).Return(&enginetypes.VirtualizationWaitResult{Message: "", Code: 0}, nil)
	e.On("VirtualizationUpdateResource", mock.Anything, mock.Anything, mock.Anything).Return(
		func(_ context.Context, ID string, params resourcetypes.Resources) error {
			fmt.Println(ID)
			litter.Dump(params)
			return nil
		},
	)
	e.On("VirtualizationCopyFrom", mock.Anything, mock.Anything, mock.Anything).Return([]byte("d1...\nd2...\n"), 0, 0, int64(0), nil)
	e.On("RawEngine", mock.Anything, mock.Anything).Return(&enginetypes.RawEngineResult{ID: "mock-raw-engine"}, nil)
	return e, nil
}
