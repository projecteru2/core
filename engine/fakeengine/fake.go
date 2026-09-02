// Package fakeengine is the in-memory engine behind mock:// nodes, for test clusters that have no runtime.
package fakeengine

import (
	"bufio"
	"context"
	"io"
	"strings"
	"time"

	"github.com/docker/go-units"

	"github.com/projecteru2/core/engine"
	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/log"
	resourcetypes "github.com/projecteru2/core/resource/types"
	coresource "github.com/projecteru2/core/source"
	coretypes "github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

const (
	// PrefixKey is the endpoint prefix for the in-memory engine.
	PrefixKey = "mock://"

	logLines = "logs1...\nlogs2...\n"
)

var _ engine.API = (*Engine)(nil)

// Engine answers every verb with canned data and forgets nothing, since it holds no state.
type Engine struct {
	params      *enginetypes.Params
	imageDigest string
}

// MakeClient builds the engine for a mock:// endpoint.
func MakeClient(_ context.Context, _ coretypes.Config, nodename, endpoint string) (engine.API, error) {
	return &Engine{params: &enginetypes.Params{Nodename: nodename, Endpoint: endpoint}, imageDigest: utils.RandomString(64)}, nil
}

func (e *Engine) Info(context.Context) (*enginetypes.Info, error) {
	return &enginetypes.Info{NCPU: 100, MemTotal: units.GiB * 100, StorageTotal: units.GiB * 100}, nil
}

func (e *Engine) Ping(context.Context) error { return nil }

func (e *Engine) CloseConn() error { return nil }

func (e *Engine) GetParams() *enginetypes.Params { return e.params }

func (e *Engine) Execute(context.Context, string, *enginetypes.ExecConfig) (string, io.ReadCloser, io.ReadCloser, io.WriteCloser, error) {
	return utils.RandomID(), stream(utils.RandomString(128)), stream(utils.RandomString(128)), sink(), nil
}

func (e *Engine) ExecResize(context.Context, string, uint, uint) error { return nil }

func (e *Engine) ExecExitCode(context.Context, string, string) (int, error) { return 0, nil }

func (e *Engine) NetworkConnect(context.Context, string, string, string, string) ([]string, error) {
	return []string{}, nil
}

func (e *Engine) NetworkDisconnect(context.Context, string, string, bool) error { return nil }

func (e *Engine) NetworkList(context.Context, []string) ([]*enginetypes.Network, error) {
	return []*enginetypes.Network{{Name: "mock-network", Subnets: []string{"1.1.1.1/8", "2.2.2.2/8"}}}, nil
}

func (e *Engine) ImageList(context.Context, string) ([]*enginetypes.Image, error) {
	return []*enginetypes.Image{{ID: "mock-image", Tags: []string{"latest"}}}, nil
}

func (e *Engine) ImageRemove(context.Context, string, bool, bool) ([]string, error) {
	return []string{"mock-image1", "mock-image2"}, nil
}

func (e *Engine) ImagesPrune(context.Context) error { return nil }

func (e *Engine) ImagePull(context.Context, string, bool) (io.ReadCloser, error) {
	return stream("pull image layer1 ...\npull image layer2...\n"), nil
}

func (e *Engine) ImagePush(context.Context, string) (io.ReadCloser, error) {
	return stream(`{"stream":"push something..."}` + "\n"), nil
}

func (e *Engine) ImageBuild(context.Context, io.Reader, []string, string) (io.ReadCloser, error) {
	return stream(`{"stream":"build something..."}` + "\n"), nil
}

func (e *Engine) ImageBuildCachePrune(context.Context, bool) (uint64, error) { return 0, nil }

func (e *Engine) ImageLocalDigests(context.Context, string) ([]string, error) {
	return []string{e.imageDigest}, nil
}

func (e *Engine) ImageRemoteDigest(context.Context, string) (string, error) {
	return e.imageDigest, nil
}

func (e *Engine) ImageBuildFromExist(context.Context, string, []string, string) (string, error) {
	return "ImageBuildFromExist", nil
}

func (e *Engine) BuildRefs(context.Context, *enginetypes.BuildRefOptions) []string {
	return []string{"ref1", "ref2"}
}

func (e *Engine) BuildContent(context.Context, coresource.Source, *enginetypes.BuildContentOptions) (string, io.Reader, error) {
	return "BuildContent", stream("this is content"), nil
}

func (e *Engine) VirtualizationCreate(ctx context.Context, opts *enginetypes.VirtualizationCreateOptions) (*enginetypes.VirtualizationCreated, error) {
	logger := log.WithFunc("engine.fakeengine.VirtualizationCreate")
	resourceOpts := &engine.VirtualizationResource{}
	if err := resourceOpts.Decode(opts.EngineParams); err != nil {
		logger.Error(ctx, err, "decode engine params")
	}
	logger.Debugf(ctx, "resources %+v", resourceOpts)
	return &enginetypes.VirtualizationCreated{ID: utils.RandomString(64), Name: "mock-test-cvm" + utils.RandomString(6)}, nil
}

func (e *Engine) VirtualizationCopyTo(context.Context, string, string, []byte, int, int, int64) error {
	return nil
}

func (e *Engine) VirtualizationCopyChunkTo(context.Context, string, string, int64, io.Reader, int, int, int64) error {
	return nil
}

func (e *Engine) VirtualizationStart(context.Context, string) error { return nil }

func (e *Engine) VirtualizationStop(context.Context, string, time.Duration) error { return nil }

func (e *Engine) VirtualizationRemove(context.Context, string, bool, bool) error { return nil }

func (e *Engine) VirtualizationSuspend(context.Context, string) error { return nil }

func (e *Engine) VirtualizationResume(context.Context, string) error { return nil }

func (e *Engine) VirtualizationInspect(_ context.Context, ID string) (*enginetypes.VirtualizationInfo, error) {
	return &enginetypes.VirtualizationInfo{ID: ID, Image: "mock-image", Running: true, Networks: map[string]string{"mock-network": "1.1.1.1"}}, nil
}

func (e *Engine) VirtualizationLogs(context.Context, *enginetypes.VirtualizationLogStreamOptions) (io.ReadCloser, io.ReadCloser, error) {
	return stream(logLines), stream(logLines), nil
}

func (e *Engine) VirtualizationAttach(context.Context, string, bool, bool) (io.ReadCloser, io.ReadCloser, io.WriteCloser, error) {
	return stream(logLines), stream(logLines), sink(), nil
}

func (e *Engine) VirtualizationResize(context.Context, string, uint, uint) error { return nil }

func (e *Engine) VirtualizationWait(context.Context, string, string) (*enginetypes.VirtualizationWaitResult, error) {
	return &enginetypes.VirtualizationWaitResult{}, nil
}

func (e *Engine) VirtualizationUpdateResource(ctx context.Context, ID string, params resourcetypes.Resources) error {
	log.WithFunc("engine.fakeengine.VirtualizationUpdateResource").WithField("ID", ID).Debugf(ctx, "resources %+v", params)
	return nil
}

func (e *Engine) VirtualizationCopyFrom(context.Context, string, string) ([]byte, int, int, int64, error) {
	return []byte("d1...\nd2...\n"), 0, 0, 0, nil
}

func (e *Engine) RawEngine(context.Context, *enginetypes.RawEngineOptions) (*enginetypes.RawEngineResult, error) {
	return &enginetypes.RawEngineResult{ID: "mock-raw-engine"}, nil
}

func (e *Engine) VerifyNode(context.Context) error { return nil }

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
