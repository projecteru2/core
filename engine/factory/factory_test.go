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
	enginetypes "github.com/projecteru2/core/engine/types"
	storemocks "github.com/projecteru2/core/store/mocks"
	"github.com/projecteru2/core/types"
)

const testPrefix = "factorytest://"

func TestNewEnginePassesParamsInDeclaredOrder(t *testing.T) {
	var got []string
	engines[testPrefix] = func(_ context.Context, _ types.Config, nodename, endpoint, ca, cert, key string) (engine.API, error) {
		got = []string{nodename, endpoint, ca, cert, key}
		return &fake.EngineWithErr{}, nil
	}
	defer delete(engines, testPrefix)

	params := &enginetypes.Params{
		Nodename: "node",
		Endpoint: testPrefix + "host",
		CA:       "ca-pem",
		Cert:     "cert-pem",
		Key:      "key-pem",
	}
	_, err := newEngine(t.Context(), types.Config{ConnectionTimeout: time.Second}, params)
	require.NoError(t, err)
	assert.Equal(t, []string{"node", testPrefix + "host", "ca-pem", "cert-pem", "key-pem"}, got)
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

func TestCheckOneNodeStatusSurvivesStoreFailure(t *testing.T) {
	params := &enginetypes.Params{Nodename: "node", Endpoint: testPrefix + "host"}
	for _, tt := range []struct {
		name    string
		status  *types.NodeStatus
		err     error
		dropped bool
	}{
		{"store failure keeps the entry", nil, errors.New("etcd unreachable"), false},
		{"missing status drops the entry", nil, types.ErrInvaildCount, true},
		{"dead node drops the entry", &types.NodeStatus{Alive: false}, nil, true},
		{"alive node keeps the entry", &types.NodeStatus{Alive: true}, nil, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			stor := storemocks.NewStore(t)
			stor.On("GetNodeStatus", mock.Anything, "node").Return(tt.status, tt.err)

			e := NewEngineCache(types.Config{MaxConcurrency: 1}, stor)
			e.Set(params.CacheKey(), &fake.EngineWithErr{})
			e.checkOneNodeStatus(t.Context(), params)
			assert.Equal(t, tt.dropped, e.Get(params.CacheKey()) == nil)
		})
	}
}

func TestCheckAliveSurvivesPoolOverload(t *testing.T) {
	block := make(chan struct{})
	e := NewEngineCache(types.Config{MaxConcurrency: 1, ConnectionTimeout: 10 * time.Millisecond}, nil)
	for i := range 2 {
		params := &enginetypes.Params{Nodename: fmt.Sprintf("node-%d", i), Endpoint: fmt.Sprintf("%shost-%d", testPrefix, i)}
		e.Set(params.CacheKey(), &blockingEngine{params: params, block: block})
	}

	ctx, cancel := context.WithCancel(t.Context())
	returned := make(chan struct{})
	go func() {
		defer close(returned)
		e.checkAlive(ctx)
	}()

	time.Sleep(100 * time.Millisecond)
	close(block)
	cancel()

	select {
	case <-returned:
	case <-time.After(5 * time.Second):
		t.Fatal("checkAlive never returned: the wait group was left unbalanced by a rejected pool task")
	}
}

type blockingEngine struct {
	engine.API
	params *enginetypes.Params
	block  chan struct{}
}

func (b *blockingEngine) GetParams() *enginetypes.Params { return b.params }

func (b *blockingEngine) Ping(context.Context) error {
	<-b.block
	return nil
}
