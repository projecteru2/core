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
		cli: cli,
	})
}

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
