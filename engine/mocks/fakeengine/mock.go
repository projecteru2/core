package fakeengine

import (
	"bufio"
	"context"
	"io"
	"strings"

	"github.com/docker/go-units"
	mock "github.com/stretchr/testify/mock"

	"github.com/projecteru2/core/engine"
	enginemocks "github.com/projecteru2/core/engine/mocks"
	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/log"
	resourcetypes "github.com/projecteru2/core/resource/types"
	coresource "github.com/projecteru2/core/source"
	coretypes "github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

const (
	// PrefixKey is the endpoint prefix for the in-memory mock engine.
	PrefixKey = "mock://"

	logLines = "logs1...\nlogs2...\n"
)

// MakeClient builds an engine.API whose calls return canned data.
func MakeClient(_ context.Context, _ coretypes.Config, nodename, endpoint string) (engine.API, error) {
	e := &enginemocks.API{}
	params := &enginetypes.Params{
		Nodename: nodename,
		Endpoint: endpoint,
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
			return stream(utils.RandomString(128))
		},
		func(context.Context, string, *enginetypes.ExecConfig) io.ReadCloser {
			return stream(utils.RandomString(128))
		},
		func(context.Context, string, *enginetypes.ExecConfig) io.WriteCloser {
			return sink()
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
	e.On("ImagePull", mock.Anything, mock.Anything, mock.Anything).Return(
		func(context.Context, string, bool) io.ReadCloser {
			return stream("pull image layer1 ...\npull image layer2...\n")
		}, nil,
	)
	e.On("ImagePush", mock.Anything, mock.Anything).Return(
		func(context.Context, string) io.ReadCloser {
			return stream(`{"stream":"push something..."}` + "\n")
		}, nil,
	)
	e.On("ImageBuild", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(
		func(context.Context, io.Reader, []string, string) io.ReadCloser {
			return stream(`{"stream":"build something..."}` + "\n")
		}, nil,
	)
	e.On("ImageBuildCachePrune", mock.Anything, mock.Anything).Return(uint64(0), nil)
	imageDigest := utils.RandomString(64)
	e.On("ImageLocalDigests", mock.Anything, mock.Anything).Return([]string{imageDigest}, nil)
	e.On("ImageRemoteDigest", mock.Anything, mock.Anything).Return(imageDigest, nil)
	e.On("ImageBuildFromExist", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("ImageBuildFromExist", nil)
	e.On("BuildRefs", mock.Anything, mock.Anything).Return([]string{"ref1", "ref2"})
	e.On("BuildContent", mock.Anything, mock.Anything, mock.Anything).Return(
		"BuildContent",
		func(context.Context, coresource.Source, *enginetypes.BuildContentOptions) io.Reader {
			return stream("this is content")
		}, nil,
	)
	e.On("VirtualizationCreate", mock.Anything, mock.Anything).Return(
		func(ctx context.Context, opts *enginetypes.VirtualizationCreateOptions) *enginetypes.VirtualizationCreated {
			logger := log.WithFunc("engine.fakeengine.VirtualizationCreate")
			resourceOpts := &engine.VirtualizationResource{}
			if err := resourceOpts.Decode(opts.EngineParams); err != nil {
				logger.Error(ctx, err, "decode engine params")
			}
			logger.Debugf(ctx, "resources %+v", resourceOpts)
			return &enginetypes.VirtualizationCreated{ID: utils.RandomString(64), Name: "mock-test-cvm" + utils.RandomString(6)}
		}, nil,
	)
	e.On("VirtualizationCopyTo", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	e.On("VirtualizationCopyChunkTo", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	e.On("VirtualizationStart", mock.Anything, mock.Anything).Return(nil)
	e.On("VirtualizationStop", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	e.On("VirtualizationRemove", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	e.On("VirtualizationSuspend", mock.Anything, mock.Anything).Return(nil)
	e.On("VirtualizationResume", mock.Anything, mock.Anything).Return(nil)
	e.On("VirtualizationInspect", mock.Anything, mock.Anything).Return(
		func(_ context.Context, ID string) *enginetypes.VirtualizationInfo {
			return &enginetypes.VirtualizationInfo{
				ID:       ID,
				Image:    "mock-image",
				Running:  true,
				Networks: map[string]string{"mock-network": "1.1.1.1"},
			}
		}, nil,
	)
	e.On("VirtualizationLogs", mock.Anything, mock.Anything).Return(
		func(context.Context, *enginetypes.VirtualizationLogStreamOptions) io.ReadCloser {
			return stream(logLines)
		},
		func(context.Context, *enginetypes.VirtualizationLogStreamOptions) io.ReadCloser {
			return stream(logLines)
		},
		nil,
	)
	e.On("VirtualizationAttach", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(
		func(context.Context, string, bool, bool) io.ReadCloser { return stream(logLines) },
		func(context.Context, string, bool, bool) io.ReadCloser { return stream(logLines) },
		func(context.Context, string, bool, bool) io.WriteCloser { return sink() },
		nil,
	)
	e.On("VirtualizationResize", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	e.On("VirtualizationWait", mock.Anything, mock.Anything, mock.Anything).Return(&enginetypes.VirtualizationWaitResult{Message: "", Code: 0}, nil)
	e.On("VirtualizationUpdateResource", mock.Anything, mock.Anything, mock.Anything).Return(
		func(ctx context.Context, ID string, params resourcetypes.Resources) error {
			log.WithFunc("engine.fakeengine.VirtualizationUpdateResource").WithField("ID", ID).Debugf(ctx, "resources %+v", params)
			return nil
		},
	)
	e.On("VirtualizationCopyFrom", mock.Anything, mock.Anything, mock.Anything).Return([]byte("d1...\nd2...\n"), 0, 0, int64(0), nil)
	e.On("RawEngine", mock.Anything, mock.Anything).Return(&enginetypes.RawEngineResult{ID: "mock-raw-engine"}, nil)
	return e, nil
}

type writeCloser struct {
	*bufio.Writer
}

func (wc *writeCloser) Close() error {
	return nil
}

func stream(content string) io.ReadCloser {
	return io.NopCloser(strings.NewReader(content))
}

func sink() io.WriteCloser {
	return &writeCloser{bufio.NewWriter(io.Discard)}
}
