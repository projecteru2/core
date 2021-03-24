package redislock

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/suite"
)

type RedisLockTestSuite struct {
	suite.Suite

	cli *redis.Client
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

func (s *RedisLockTestSuite) TestTryLock() {
	l1, err := New(s.cli, "test", time.Second, time.Second)
	s.NoError(err)
	l2, err := New(s.cli, "test", time.Second, time.Second)
	s.NoError(err)

	ctx1, err := l1.Lock(context.Background())
	s.Nil(ctx1.Err())
	s.NoError(err)

	ctx2, err := l2.TryLock(context.Background())
	s.Nil(ctx2)
	s.Error(err)
}

func TestRedisLock(t *testing.T) {
	addr := os.Getenv("REDIS_HOST")
	if addr == "" {
		addr = "localhost:6379"
	}
	cli := redis.NewClient(&redis.Options{
		Addr: addr,
		DB:   0,
	})
	defer cli.Close()
	suite.Run(t, &RedisLockTestSuite{
		cli: cli,
	})
}
