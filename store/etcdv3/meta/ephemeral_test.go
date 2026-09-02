package meta

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestEphemeralDeregister(t *testing.T) {
	m := NewEmbeddedETCD(t)

	ctx := t.Context()
	path := "/ident"
	heartbeat := time.Millisecond
	expiry, stop, err := m.StartEphemeral(ctx, path, heartbeat)
	require.NoError(t, err)
	require.NotNil(t, stop)
	require.NotNil(t, expiry)

	kv, err := m.GetOne(ctx, path)
	require.NoError(t, err)
	require.Equal(t, path, string(kv.Key))

	stop()
	kv, err = m.GetOne(ctx, path)
	require.Error(t, err)
	require.Nil(t, kv)
	select {
	case <-expiry:
	case <-time.After(time.Second * 8):
		require.FailNow(t, path+" should expire after stop")
	}
}

func TestEphemeral(t *testing.T) {
	m := NewEmbeddedETCD(t)

	ctx := t.Context()
	path := "/ident"
	heartbeat := time.Millisecond
	expiry, stop, err := m.StartEphemeral(ctx, path, heartbeat)
	require.NoError(t, err)
	require.NotNil(t, stop)
	require.NotNil(t, expiry)

	kv, err := m.GetOne(ctx, path)
	require.NoError(t, err)
	require.Equal(t, path, string(kv.Key))

	time.Sleep(heartbeat * 5)
	kv, err = m.GetOne(ctx, path)
	require.NoError(t, err)
	require.Equal(t, path, string(kv.Key))

	select {
	case <-expiry:
		require.FailNow(t, "unexpected expired")
	default:
	}

	stop()
	time.Sleep(heartbeat * 5)
	kv, err = m.GetOne(ctx, path)
	require.Error(t, err)
	require.Nil(t, kv)

	select {
	case <-expiry:
	default:
		require.FailNow(t, "expected expired")
	}
}

func TestEphemeralFailedAsPutAlready(t *testing.T) {
	m := NewEmbeddedETCD(t)

	ctx := t.Context()
	path := "/ident"
	heartbeat := time.Millisecond
	expiry, stop, err := m.StartEphemeral(ctx, path, heartbeat)
	require.NoError(t, err)
	require.NotNil(t, stop)
	require.NotNil(t, expiry)

	defer stop()

	_, _, err = m.StartEphemeral(ctx, path, heartbeat)
	require.Error(t, err)

	leases, err := m.cliv3.Leases(ctx)
	require.NoError(t, err)
	require.Len(t, leases.Leases, 1)
}
