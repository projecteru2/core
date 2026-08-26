package wal

import (
	"fmt"
	"maps"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/projecteru2/core/store/etcdv3"
	"github.com/projecteru2/core/store/etcdv3/embedded"
	"github.com/projecteru2/core/types"
)

func TestLogFailedAsNoSuchHandler(t *testing.T) {
	hydro := newTestHydro(t, newMemStore())
	commit, err := hydro.Log("create", struct{}{})
	assert.Error(t, err)
	assert.Nil(t, commit)
}

func TestLogFailedAsEncodeError(t *testing.T) {
	var handled, encoded, decoded bool
	eventype := "create"
	handler := newTestEventHandler(eventype, &handled, &encoded, &decoded)
	handler.encode = func(any) ([]byte, error) { return nil, fmt.Errorf("encode error") }

	hydro := newTestHydro(t, newMemStore())
	hydro.Register(handler)

	commit, err := hydro.Log(eventype, struct{}{})
	assert.Error(t, err)
	assert.Nil(t, commit)
	assert.False(t, encoded)
	assert.False(t, decoded)
	assert.False(t, handled)
}

func TestLogFailedAsStoreError(t *testing.T) {
	var handled, encoded, decoded bool
	eventype := "create"
	handler := newTestEventHandler(eventype, &handled, &encoded, &decoded)

	store := newMemStore()
	hydro := newTestHydro(t, store)
	hydro.Register(handler)
	store.putErr = fmt.Errorf("put error")

	commit, err := hydro.Log(eventype, struct{}{})
	assert.Error(t, err)
	assert.Nil(t, commit)
}

func TestLogWithCommitEvent(t *testing.T) {
	var handled, encoded, decoded bool
	eventype := "create"
	handler := newTestEventHandler(eventype, &handled, &encoded, &decoded)

	store := newMemStore()
	hydro := newTestHydro(t, store)
	hydro.Register(handler)

	commit, err := hydro.Log(eventype, struct{}{})
	require.NoError(t, err)
	require.NotNil(t, commit)
	assert.Len(t, store.data, 1)

	assert.NoError(t, commit())
	assert.Empty(t, store.data)
	assert.True(t, encoded)
	assert.False(t, decoded)
	assert.False(t, handled)
}

func TestRecoverFailedAsNoSuchHandler(t *testing.T) {
	var handled, encoded, decoded bool
	eventype := "create"
	handler := newTestEventHandler(eventype, &handled, &encoded, &decoded)

	store := newMemStore()
	hydro := newTestHydro(t, store)
	hydro.Register(handler)

	_, err := hydro.Log(eventype, struct{}{})
	require.NoError(t, err)
	hydro.handlers.Delete(eventype)

	hydro.Recover(t.Context())
	assert.True(t, encoded)
	assert.False(t, decoded)
	assert.False(t, handled)
	assert.Len(t, store.data, 1)
}

func TestRecoverFailedAsDecodeEventError(t *testing.T) {
	var handled, encoded, decoded bool
	eventype := "create"
	handler := newTestEventHandler(eventype, &handled, &encoded, &decoded)

	store := newMemStore()
	hydro := newTestHydro(t, store)
	hydro.Register(handler)
	store.data[fmt.Sprintf(eventKey, hydro.address, 1)] = "not an event"

	hydro.Recover(t.Context())
	assert.False(t, decoded)
	assert.False(t, handled)
	assert.Len(t, store.data, 1)
}

func TestRecoverFailedAsReadError(t *testing.T) {
	var handled, encoded, decoded bool
	eventype := "create"
	handler := newTestEventHandler(eventype, &handled, &encoded, &decoded)

	store := newMemStore()
	hydro := newTestHydro(t, store)
	hydro.Register(handler)

	_, err := hydro.Log(eventype, struct{}{})
	require.NoError(t, err)
	store.getErr = fmt.Errorf("read error")

	hydro.Recover(t.Context())
	assert.False(t, decoded)
	assert.False(t, handled)
}

func TestRecoverFailedAsDecodeLogError(t *testing.T) {
	var handled, encoded, decoded bool
	eventype := "create"
	handler := newTestEventHandler(eventype, &handled, &encoded, &decoded)
	handler.decode = func([]byte) (any, error) {
		decoded = true
		return nil, fmt.Errorf("decode error")
	}

	store := newMemStore()
	hydro := newTestHydro(t, store)
	hydro.Register(handler)

	_, err := hydro.Log(eventype, struct{}{})
	require.NoError(t, err)

	hydro.Recover(t.Context())
	assert.True(t, encoded)
	assert.True(t, decoded)
	assert.False(t, handled)
	assert.Len(t, store.data, 1)
}

func TestHydroRecover(t *testing.T) {
	var handled, encoded, decoded bool
	eventype := "create"
	handler := newTestEventHandler(eventype, &handled, &encoded, &decoded)

	store := newMemStore()
	hydro := newTestHydro(t, store)
	hydro.Register(handler)

	_, err := hydro.Log(eventype, struct{}{})
	require.NoError(t, err)

	hydro.Recover(t.Context())
	assert.True(t, encoded)
	assert.True(t, decoded)
	assert.True(t, handled)
	assert.Empty(t, store.data)
}

func TestHydroKeepsOtherAddressesAlone(t *testing.T) {
	var handled, encoded, decoded bool
	eventype := "create"
	handler := newTestEventHandler(eventype, &handled, &encoded, &decoded)

	store := newMemStore()
	hydro := newTestHydro(t, store)
	hydro.Register(handler)

	_, err := hydro.Log(eventype, struct{}{})
	require.NoError(t, err)
	peer := fmt.Sprintf(eventKey, "10.0.0.2:5001", 1)
	store.data[peer] = store.data[fmt.Sprintf(eventKey, hydro.address, 1)]

	hydro.Recover(t.Context())
	assert.True(t, handled)
	assert.Equal(t, []string{peer}, keysOf(store))
}

func TestHydroWithRealStore(t *testing.T) {
	address := "10.0.0.1:5001"
	ctx := t.Context()
	store := newTestStore(t)

	handled := []string{}
	handler := simpleEventHandler{
		event:  "create",
		encode: func(raw any) ([]byte, error) { return []byte(raw.(string)), nil },
		decode: func(bs []byte) (any, error) { return string(bs), nil },
		handle: func(raw any) error { handled = append(handled, raw.(string)); return nil },
	}

	hydro, err := NewHydro(ctx, store, address, testConfig())
	require.NoError(t, err)
	hydro.Register(handler)
	for _, name := range []string{"first", "second", "third"} {
		_, err = hydro.Log(handler.event, name)
		require.NoError(t, err)
	}

	logged, err := store.GetPrefix(ctx, fmt.Sprintf(addressPrefix, address), 0)
	require.NoError(t, err)
	require.Len(t, logged, 3)
	require.Contains(t, logged, fmt.Sprintf(eventKey, address, 1))

	restarted, err := NewHydro(ctx, store, address, testConfig())
	require.NoError(t, err)
	restarted.Register(handler)
	_, err = restarted.Log(handler.event, "fourth")
	require.NoError(t, err)

	restarted.Recover(ctx)
	assert.Equal(t, []string{"first", "second", "third", "fourth"}, handled)

	left, err := store.GetPrefix(ctx, fmt.Sprintf(addressPrefix, address), 0)
	require.NoError(t, err)
	assert.Empty(t, left)
}

func TestTakeoverReplaysAnUnregisteredInstance(t *testing.T) {
	var handled, encoded, decoded bool
	eventype := "create"
	handler := newTestEventHandler(eventype, &handled, &encoded, &decoded)

	store := newMemStore()
	hydro := newTestHydro(t, store)
	hydro.Register(handler)

	_, err := hydro.Log(eventype, struct{}{})
	require.NoError(t, err)
	mine := fmt.Sprintf(eventKey, hydro.address, 1)
	dead := fmt.Sprintf(eventKey, "10.0.0.9:5001", 1)
	store.data[dead] = store.data[mine]

	hydro.Takeover(t.Context(), []string{hydro.address})
	assert.True(t, handled)
	assert.Equal(t, []string{mine}, keysOf(store))
}

func TestTakeoverLeavesRegisteredInstancesAlone(t *testing.T) {
	var handled, encoded, decoded bool
	eventype := "create"
	handler := newTestEventHandler(eventype, &handled, &encoded, &decoded)

	store := newMemStore()
	hydro := newTestHydro(t, store)
	hydro.Register(handler)

	_, err := hydro.Log(eventype, struct{}{})
	require.NoError(t, err)
	peer := "10.0.0.9:5001"
	store.data[fmt.Sprintf(eventKey, peer, 1)] = store.data[fmt.Sprintf(eventKey, hydro.address, 1)]

	hydro.Takeover(t.Context(), []string{hydro.address, peer})
	assert.False(t, handled)
	assert.Len(t, store.data, 2)
}

func TestTakeoverWaitsForTheLiveInstances(t *testing.T) {
	var handled, encoded, decoded bool
	eventype := "create"
	handler := newTestEventHandler(eventype, &handled, &encoded, &decoded)

	store := newMemStore()
	hydro := newTestHydro(t, store)
	hydro.Register(handler)

	value, err := NewHydroEvent(eventype, []byte("{}")).Encode()
	require.NoError(t, err)
	dead := fmt.Sprintf(eventKey, "10.0.0.9:5001", 1)
	store.data[dead] = string(value)

	hydro.Takeover(t.Context(), nil)
	assert.False(t, handled)
	assert.Equal(t, []string{dead}, keysOf(store))
}

func TestTakeoverFailedAsNoLock(t *testing.T) {
	var handled, encoded, decoded bool
	eventype := "create"
	handler := newTestEventHandler(eventype, &handled, &encoded, &decoded)

	store := newMemStore()
	hydro := newTestHydro(t, store)
	hydro.Register(handler)

	value, err := NewHydroEvent(eventype, []byte("{}")).Encode()
	require.NoError(t, err)
	store.data[fmt.Sprintf(eventKey, "10.0.0.9:5001", 1)] = string(value)
	store.lockErr = fmt.Errorf("lock error")

	hydro.Takeover(t.Context(), []string{hydro.address})
	assert.False(t, handled)
	assert.Len(t, store.data, 1)
}

func newTestHydro(t *testing.T, store Store) *Hydro {
	hydro, err := NewHydro(t.Context(), store, "10.0.0.1:5001", testConfig())
	require.NoError(t, err)
	return hydro
}

func testConfig() types.Config {
	return types.Config{GlobalTimeout: time.Minute, LockTimeout: 10 * time.Second}
}

func newTestStore(t *testing.T) *etcdv3.Mercury {
	cluster, err := embedded.New(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(cluster.Close)

	config := types.Config{MaxConcurrency: 10, LockTimeout: 10 * time.Second, GlobalTimeout: 30 * time.Second}
	config.Etcd = types.EtcdConfig{Machines: []string{"127.0.0.1:2379"}, Prefix: "/eru-test", LockPrefix: "/eru-test-lock"}
	store, err := etcdv3.New(config, cluster)
	require.NoError(t, err)
	return store
}

func newTestEventHandler(eventype string, handled, encoded, decoded *bool) simpleEventHandler {
	handle := func(any) (err error) {
		*handled = true
		return err
	}

	encode := func(any) (bs []byte, err error) {
		*encoded = true
		return bs, err
	}

	decode := func([]byte) (item any, err error) {
		*decoded = true
		return item, err
	}

	return simpleEventHandler{
		event:  eventype,
		encode: encode,
		decode: decode,
		handle: handle,
	}
}

func keysOf(store *memStore) []string {
	store.Lock()
	defer store.Unlock()
	return slices.Sorted(maps.Keys(store.data))
}
