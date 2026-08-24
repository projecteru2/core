package fakeengine

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/projecteru2/core/engine"
	enginetypes "github.com/projecteru2/core/engine/types"
	resourcetypes "github.com/projecteru2/core/resource/types"
	coretypes "github.com/projecteru2/core/types"
)

func TestEveryAPIMethodIsAnswered(t *testing.T) {
	api, err := MakeClient(t.Context(), coretypes.Config{}, "node", PrefixKey+"host", "", "", "")
	require.NoError(t, err)

	for _, tt := range apiCalls() {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotPanics(t, func() { tt.call(t.Context(), api) })
		})
	}
}

func TestStreamsAreFreshOnEveryCall(t *testing.T) {
	api, err := MakeClient(t.Context(), coretypes.Config{}, "node", PrefixKey+"host", "", "", "")
	require.NoError(t, err)

	for _, tt := range []struct {
		name string
		read func(context.Context) (io.ReadCloser, error)
	}{
		{"ImagePull", func(ctx context.Context) (io.ReadCloser, error) { return api.ImagePull(ctx, "img", false) }},
		{"ImagePush", func(ctx context.Context) (io.ReadCloser, error) { return api.ImagePush(ctx, "img") }},
		{"ImageBuild", func(ctx context.Context) (io.ReadCloser, error) {
			return api.ImageBuild(ctx, strings.NewReader(""), nil, "")
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			for i := range 2 {
				rc, err := tt.read(t.Context())
				require.NoError(t, err)
				body, err := io.ReadAll(rc)
				require.NoError(t, err)
				assert.NotEmpty(t, body, "call %d returned an exhausted stream", i+1)
			}
		})
	}
}

func TestVirtualizationInspectReportsTheRequestedID(t *testing.T) {
	api, err := MakeClient(t.Context(), coretypes.Config{}, "node", PrefixKey+"host", "", "", "")
	require.NoError(t, err)

	info, err := api.VirtualizationInspect(t.Context(), "workload-id")
	require.NoError(t, err)
	assert.Equal(t, "workload-id", info.ID)
}

func apiCalls() []struct {
	name string
	call func(context.Context, engine.API)
} {
	return []struct {
		name string
		call func(context.Context, engine.API)
	}{
		{"Info", func(ctx context.Context, a engine.API) { a.Info(ctx) }},                                                          //nolint
		{"Ping", func(ctx context.Context, a engine.API) { a.Ping(ctx) }},                                                          //nolint
		{"CloseConn", func(_ context.Context, a engine.API) { a.CloseConn() }},                                                     //nolint
		{"GetParams", func(_ context.Context, a engine.API) { a.GetParams() }},                                                     //nolint
		{"Execute", func(ctx context.Context, a engine.API) { a.Execute(ctx, "id", nil) }},                                         //nolint
		{"ExecResize", func(ctx context.Context, a engine.API) { a.ExecResize(ctx, "e", 1, 1) }},                                   //nolint
		{"ExecExitCode", func(ctx context.Context, a engine.API) { a.ExecExitCode(ctx, "id", "e") }},                               //nolint
		{"NetworkConnect", func(ctx context.Context, a engine.API) { a.NetworkConnect(ctx, "n", "t", "", "") }},                    //nolint
		{"NetworkDisconnect", func(ctx context.Context, a engine.API) { a.NetworkDisconnect(ctx, "n", "t", false) }},               //nolint
		{"NetworkList", func(ctx context.Context, a engine.API) { a.NetworkList(ctx, nil) }},                                       //nolint
		{"ImageList", func(ctx context.Context, a engine.API) { a.ImageList(ctx, "i") }},                                           //nolint
		{"ImageRemove", func(ctx context.Context, a engine.API) { a.ImageRemove(ctx, "i", false, false) }},                         //nolint
		{"ImagesPrune", func(ctx context.Context, a engine.API) { a.ImagesPrune(ctx) }},                                            //nolint
		{"ImagePull", func(ctx context.Context, a engine.API) { a.ImagePull(ctx, "i", false) }},                                    //nolint
		{"ImagePush", func(ctx context.Context, a engine.API) { a.ImagePush(ctx, "i") }},                                           //nolint
		{"ImageBuild", func(ctx context.Context, a engine.API) { a.ImageBuild(ctx, strings.NewReader(""), nil, "") }},              //nolint
		{"ImageBuildCachePrune", func(ctx context.Context, a engine.API) { a.ImageBuildCachePrune(ctx, true) }},                    //nolint
		{"ImageLocalDigests", func(ctx context.Context, a engine.API) { a.ImageLocalDigests(ctx, "i") }},                           //nolint
		{"ImageRemoteDigest", func(ctx context.Context, a engine.API) { a.ImageRemoteDigest(ctx, "i") }},                           //nolint
		{"ImageBuildFromExist", func(ctx context.Context, a engine.API) { a.ImageBuildFromExist(ctx, "id", []string{"r"}, "u") }},  //nolint
		{"BuildRefs", func(ctx context.Context, a engine.API) { a.BuildRefs(ctx, &enginetypes.BuildRefOptions{}) }},                //nolint
		{"BuildContent", func(ctx context.Context, a engine.API) { a.BuildContent(ctx, nil, &enginetypes.BuildContentOptions{}) }}, //nolint
		{"VirtualizationCreate", func(ctx context.Context, a engine.API) {
			a.VirtualizationCreate(ctx, &enginetypes.VirtualizationCreateOptions{}) //nolint
		}},
		{"VirtualizationCopyTo", func(ctx context.Context, a engine.API) { a.VirtualizationCopyTo(ctx, "id", "/t", nil, 0, 0, 0) }}, //nolint
		{"VirtualizationCopyChunkTo", func(ctx context.Context, a engine.API) {
			a.VirtualizationCopyChunkTo(ctx, "id", "/t", 0, strings.NewReader(""), 0, 0, 0) //nolint
		}},
		{"VirtualizationStart", func(ctx context.Context, a engine.API) { a.VirtualizationStart(ctx, "id") }},               //nolint
		{"VirtualizationStop", func(ctx context.Context, a engine.API) { a.VirtualizationStop(ctx, "id", time.Second) }},    //nolint
		{"VirtualizationRemove", func(ctx context.Context, a engine.API) { a.VirtualizationRemove(ctx, "id", true, true) }}, //nolint
		{"VirtualizationSuspend", func(ctx context.Context, a engine.API) { a.VirtualizationSuspend(ctx, "id") }},           //nolint
		{"VirtualizationResume", func(ctx context.Context, a engine.API) { a.VirtualizationResume(ctx, "id") }},             //nolint
		{"VirtualizationInspect", func(ctx context.Context, a engine.API) { a.VirtualizationInspect(ctx, "id") }},           //nolint
		{"VirtualizationLogs", func(ctx context.Context, a engine.API) {
			a.VirtualizationLogs(ctx, &enginetypes.VirtualizationLogStreamOptions{}) //nolint
		}},
		{"VirtualizationAttach", func(ctx context.Context, a engine.API) { a.VirtualizationAttach(ctx, "id", true, true) }}, //nolint
		{"VirtualizationResize", func(ctx context.Context, a engine.API) { a.VirtualizationResize(ctx, "id", 1, 1) }},       //nolint
		{"VirtualizationWait", func(ctx context.Context, a engine.API) { a.VirtualizationWait(ctx, "id", "s") }},            //nolint
		{"VirtualizationUpdateResource", func(ctx context.Context, a engine.API) {
			a.VirtualizationUpdateResource(ctx, "id", resourcetypes.Resources{}) //nolint
		}},
		{"VirtualizationCopyFrom", func(ctx context.Context, a engine.API) { a.VirtualizationCopyFrom(ctx, "id", "/p") }}, //nolint
		{"RawEngine", func(ctx context.Context, a engine.API) { a.RawEngine(ctx, &enginetypes.RawEngineOptions{}) }},      //nolint
	}
}
