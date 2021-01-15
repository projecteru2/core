package meta

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestEphemeral(t *testing.T) {
	m := NewEmbeddedETCD(t)
	defer m.TerminateEmbededStorage()

	ctx := context.Background()
	path := "/ident"
	heartbeat := time.Millisecond
	expiry, stop, err := m.StartEphemeral(ctx, path, heartbeat)
	require.NoError(t, err)
	require.NotNil(t, stop)
	require.NotNil(t, expiry)

	kv, err := m.GetOne(ctx, path)
	require.NoError(t, err)
	require.Equal(t, path, string(kv.Key))

	// Makes sure that the ephemeral keeps alived.
	time.Sleep(heartbeat * 5)
	kv, err = m.GetOne(ctx, path)
	require.NoError(t, err)
	require.Equal(t, path, string(kv.Key))

	select {
	case <-expiry:
		require.FailNow(t, "unexpected expired")
	default:
	}

	// Stop and waiting for expiry.
	stop()
	time.Sleep(heartbeat * 5)
	// Ephemeral kv has been removed.
	kv, err = m.GetOne(ctx, path)
	require.Error(t, err) // no such path
	require.Nil(t, kv)

	select {
	case <-expiry:
	default:
		require.FailNow(t, "expected expired")
	}
}

func TestEphemeralFailedAsPutAlready(t *testing.T) {
	m := NewEmbeddedETCD(t)
	defer m.TerminateEmbededStorage()

	ctx := context.Background()
	path := "/ident"
	heartbeat := time.Millisecond
	expiry, stop, err := m.StartEphemeral(ctx, path, heartbeat)
	require.NoError(t, err)
	require.NotNil(t, stop)
	require.NotNil(t, expiry)

	defer stop()

	_, _, err = m.StartEphemeral(ctx, path, heartbeat)
	require.Error(t, err)
}

func TestEphemeralMustRevokeAfterKeepaliveFailed(t *testing.T) {
	m := NewEmbeddedETCD(t)

	ctx := context.Background()
	path := "/ident"
	expiry, stop, err := m.StartEphemeral(ctx, path, time.Millisecond)
	require.NoError(t, err)
	require.NotNil(t, stop)
	require.NotNil(t, expiry)

	// Triggers keepalive failed.
	m.TerminateEmbededStorage()

	select {
	case <-expiry:
	case <-time.After(time.Second * 8):
		require.FailNow(t, "%s should had been removed", path)
	}
}
