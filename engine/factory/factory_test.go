package factory

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/projecteru2/core/engine"
	"github.com/projecteru2/core/engine/fake"
	enginemocks "github.com/projecteru2/core/engine/mocks"
	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/types"
)

const testPrefix = "factorytest://"

func TestNewEnginePassesParamsInDeclaredOrder(t *testing.T) {
	var got []string
	engines[testPrefix] = func(_ context.Context, _ types.Config, nodename, endpoint string) (engine.API, error) {
		got = []string{nodename, endpoint}
		return &fake.EngineWithErr{}, nil
	}
	defer delete(engines, testPrefix)

	params := &enginetypes.Params{
		Nodename: "node",
		Endpoint: testPrefix + "host",
	}
	_, err := newEngine(t.Context(), types.Config{ConnectionTimeout: time.Second}, params)
	require.NoError(t, err)
	assert.Equal(t, []string{"node", testPrefix + "host"}, got)
}

func TestNewEngineClosesAnEngineThatCannotAnswer(t *testing.T) {
	unreachable := &enginemocks.API{}
	unreachable.On("Ping", mock.Anything).Return(errors.New("containerd is down"))
	unreachable.On("CloseConn").Return(nil)
	engines[testPrefix] = func(context.Context, types.Config, string, string) (engine.API, error) {
		return unreachable, nil
	}
	defer delete(engines, testPrefix)

	params := &enginetypes.Params{Nodename: "node", Endpoint: testPrefix + "host"}
	_, err := newEngine(t.Context(), types.Config{ConnectionTimeout: time.Second}, params)

	require.Error(t, err)
	unreachable.AssertCalled(t, "CloseConn")
}

func TestGetReturnsNilAfterDelete(t *testing.T) {
	e := NewEngineCache(types.Config{MaxConcurrency: 1}, nil)
	e.Set("k", &fake.EngineWithErr{})
	require.NotNil(t, e.Get("k"))

	e.Delete("k")
	assert.Nil(t, e.Get("k"))
}

func TestGetReturnsNilForEveryDeletedKey(t *testing.T) {
	e := NewEngineCache(types.Config{MaxConcurrency: 1}, nil)
	keys := make([]string, 0, 64)
	for i := range 64 {
		key := fmt.Sprintf("tcp://10.0.0.%d:2376-cafebabe", i)
		keys = append(keys, key)
		e.Set(key, &fake.EngineWithErr{})
	}
	for _, key := range keys {
		e.Delete(key)
		assert.Nil(t, e.Get(key), "deleted engine %s is still served from the cache", key)
	}
}
