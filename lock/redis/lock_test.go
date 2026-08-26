package redislock

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/suite"
)

func TestRedisLock(t *testing.T) {
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
	suite.Run(t, &RedisLockTestSuite{
		cli:    cli,
		server: s,
	})
}

type RedisLockTestSuite struct {
	suite.Suite

	cli    *redis.Client
	server *miniredis.Miniredis
}

func (s *RedisLockTestSuite) SetupTest() {
	s.cli.FlushAll(context.Background())
}

func (s *RedisLockTestSuite) TearDownTest() {
	s.cli.FlushAll(context.Background())
}

func (s *RedisLockTestSuite) TestMutex() {
	_, err := New(s.cli, "", time.Second, time.Second)
	s.Error(err)
	l, err := New(s.cli, "test", time.Second, time.Second)
	s.NoError(err)

	ctx := context.Background()
	ctx, err = l.Lock(ctx)
	s.Nil(ctx.Err())
	s.NoError(err)

	err = l.Unlock(ctx)
	s.NoError(err)
}

func (s *RedisLockTestSuite) TestLostLeaseCancelsContext() {
	l, err := New(s.cli, "test", time.Second, 90*time.Millisecond)
	s.Require().NoError(err)

	ctx, err := l.Lock(context.Background())
	s.Require().NoError(err)
	s.server.Del("/test")

	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		s.FailNow("lock context was not canceled")
	}
	s.Error(l.Unlock(context.Background()))
}

func (s *RedisLockTestSuite) TestTransientRefreshErrorKeepsContext() {
	l, err := New(s.cli, "test", time.Second, 90*time.Millisecond)
	s.Require().NoError(err)

	ctx, err := l.Lock(context.Background())
	s.Require().NoError(err)
	s.server.SetError("mock outage")

	select {
	case <-ctx.Done():
		s.FailNow("transient refresh error canceled the lock context")
	case <-time.After(200 * time.Millisecond):
	}
	s.server.SetError("")
	s.NoError(l.Unlock(context.Background()))
}
