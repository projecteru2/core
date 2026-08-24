package factory

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/projecteru2/core/engine"
	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/engine/fake"
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
