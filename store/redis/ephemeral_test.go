package redis

import (
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"

	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

func TestEphemeralMustRevokeAfterKeepaliveFailed(t *testing.T) {
	assert := assert.New(t)

	s, err := miniredis.Run()
	if err != nil {
		t.Fail()
	}
	defer s.Close()

	cli := redis.NewClient(&redis.Options{
		Addr: s.Addr(),
		DB:   0,
	})
	defer cli.Close()

	pool, _ := utils.NewPool(10000)

	rediaron := newRediaron(cli, types.Config{}, pool)

	ctx := t.Context()
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

func (s *RediaronTestSuite) TestEphemeralDeregister() {
	ctx := s.T().Context()
	path := "/ident"
	heartbeat := time.Second
	expiry, stop, err := s.rediaron.StartEphemeral(ctx, path, heartbeat)
	s.NoError(err)
	s.NotNil(stop)
	s.NotNil(expiry)

	v, err := s.rediaron.GetOne(ctx, path)
	s.NoError(err)
	s.NotEmpty(v)

	stop()
	v, err = s.rediaron.GetOne(ctx, path)
	s.Error(err)
	s.Empty(v)
}

func (s *RediaronTestSuite) TestEphemeral() {
	ctx := s.T().Context()
	path := "/ident"
	heartbeat := time.Second
	expiry, stop, err := s.rediaron.StartEphemeral(ctx, path, heartbeat)
	s.NoError(err)
	s.NotNil(stop)
	s.NotNil(expiry)

	v, err := s.rediaron.GetOne(ctx, path)
	s.NoError(err)
	s.NotEmpty(v)

	time.Sleep(heartbeat * 2)
	v, err = s.rediaron.GetOne(ctx, path)
	s.NoError(err)
	s.NotEmpty(v)

	select {
	case <-expiry:
		s.FailNow("unexpected expired")
	default:
	}

	stop()
	time.Sleep(heartbeat * 2)
	v, err = s.rediaron.GetOne(ctx, path)
	s.Error(err)
	s.Empty(v)

	select {
	case <-expiry:
	default:
		s.FailNow("expected expired")
	}
}

func (s *RediaronTestSuite) TestEphemeralFailedAsPutAlready() {
	ctx := s.T().Context()
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

func (s *RediaronTestSuite) TestEphemeralStopsAfterOwnershipChanges() {
	ctx := s.T().Context()
	path := "/ident"
	heartbeat := 90 * time.Millisecond
	expiry, stop, err := s.rediaron.StartEphemeral(ctx, path, heartbeat)
	s.Require().NoError(err)

	replacement := utils.RandomID()
	s.Require().NoError(s.rediaron.cli.Set(ctx, path, replacement, time.Second).Err())

	select {
	case <-expiry:
	case <-time.After(time.Second):
		s.FailNow("lease ownership loss was not reported")
	}
	stop()

	value, err := s.rediaron.GetOne(ctx, path)
	s.NoError(err)
	s.Equal(replacement, value)
}
