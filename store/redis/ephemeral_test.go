package redis

import (
	"context"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
)

func (s *RediaronTestSuite) TestEphemeralDeregister() {
	ctx := context.Background()
	path := "/ident"
	heartbeat := time.Second
	expiry, stop, err := s.rediaron.StartEphemeral(ctx, path, heartbeat)
	s.NoError(err)
	s.NotNil(stop)
	s.NotNil(expiry)

	v, err := s.rediaron.GetOne(ctx, path)
	s.NoError(err)
	s.Equal(ephemeralValue, v)

	stop()
	v, err = s.rediaron.GetOne(ctx, path)
	s.Error(err)
	s.Empty(v)
}

func (s *RediaronTestSuite) TestEphemeral() {
	ctx := context.Background()
	path := "/ident"
	heartbeat := time.Second
	expiry, stop, err := s.rediaron.StartEphemeral(ctx, path, heartbeat)
	s.NoError(err)
	s.NotNil(stop)
	s.NotNil(expiry)

	v, err := s.rediaron.GetOne(ctx, path)
	s.NoError(err)
	s.Equal(ephemeralValue, v)

	// Makes sure that the ephemeral keeps alived.
	time.Sleep(heartbeat * 2)
	v, err = s.rediaron.GetOne(ctx, path)
	s.NoError(err)
	s.Equal(ephemeralValue, v)

	select {
	case <-expiry:
		s.FailNow("unexpected expired")
	default:
	}

	// Stop and waiting for expiry.
	stop()
	time.Sleep(heartbeat * 2)
	// Ephemeral kv has been removed.
	v, err = s.rediaron.GetOne(ctx, path)
	s.Error(err) // no such path
	s.Empty(v)

	select {
	case <-expiry:
	default:
		s.FailNow("expected expired")
	}
}

func (s *RediaronTestSuite) TestEphemeralFailedAsPutAlready() {
	ctx := context.Background()
	path := "/ident"
	heartbeat := time.Second
	expiry, stop, err := s.rediaron.StartEphemeral(ctx, path, heartbeat)
	s.NoError(err)
	s.NotNil(stop)
	s.NotNil(expiry)

	defer stop()

	_, _, err = s.rediaron.StartEphemeral(ctx, path, heartbeat)
	s.Error(err)
}

func TestEphemeralMustRevokeAfterKeepaliveFailed(t *testing.T) {
	assert := assert.New(t)
	cli := redis.NewClient(&redis.Options{
		Addr: getRedisHost(),
		DB:   0,
	})
	defer cli.Close()

	rediaron := &Rediaron{
		cli: cli,
	}

	ctx := context.Background()
	path := "/ident"
	expiry, stop, err := rediaron.StartEphemeral(ctx, path, time.Millisecond)

	assert.NoError(err)
	assert.NotNil(stop)
	assert.NotNil(expiry)

	cli.Close()

	select {
	case <-expiry:
	case <-time.After(time.Second * 8):
		assert.FailNow("%s should had been removed", path)
	}
}
