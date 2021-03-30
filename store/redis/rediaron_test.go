package redis

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/projecteru2/core/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type RediaronTestSuite struct {
	suite.Suite

	rediaron *Rediaron
}

func (s *RediaronTestSuite) SetupTest() {
	s.rediaron.cli.FlushAll(context.Background())
}

func (s *RediaronTestSuite) TearDownTest() {
	s.rediaron.cli.FlushAll(context.Background())
}

func (s *RediaronTestSuite) TestKeyNotify() {
	ctx, cancel := context.WithCancel(context.Background())
	ch := s.rediaron.KNotify(ctx, "a*")
	go func() {
		time.Sleep(2 * time.Second)
		cancel()
	}()

	time.Sleep(time.Second)
	s.rediaron.cli.Set(context.Background(), "aaa", 1, 0)
	s.rediaron.cli.Set(context.Background(), "aab", 1, 0)
	s.rediaron.cli.Set(context.Background(), "bab", 1, 0)
	s.rediaron.cli.Del(context.Background(), "aaa")

	messages := []*KNotifyMessage{}
	for m := range ch {
		messages = append(messages, m)
	}

	s.Equal(messages[0].Key, "aaa")
	s.Equal(messages[0].Action, "set")
	s.Equal(messages[1].Key, "aab")
	s.Equal(messages[1].Action, "set")
	s.Equal(messages[2].Key, "aaa")
	s.Equal(messages[2].Action, "del")
}

func getRedisHost() string {
	addr := os.Getenv("REDIS_HOST")
	if addr == "" {
		return "localhost:6379"
	}
	return addr
}

func TestRediaron(t *testing.T) {
	config := types.Config{}
	config.LockTimeout = 10 * time.Second
	config.GlobalTimeout = 30 * time.Second

	cli := redis.NewClient(&redis.Options{
		Addr: getRedisHost(),
		DB:   0,
	})
	defer cli.Close()
	suite.Run(t, &RediaronTestSuite{
		rediaron: &Rediaron{
			cli:    cli,
			config: config,
		},
	})
}

func TestTerminateEmbeddedStorage(t *testing.T) {
	cli := redis.NewClient(&redis.Options{
		Addr: getRedisHost(),
		DB:   0,
	})
	defer cli.Close()

	rediaron := &Rediaron{
		cli: cli,
	}

	_, err := rediaron.cli.Ping(context.Background()).Result()
	assert.NoError(t, err)

	rediaron.TerminateEmbededStorage()
	_, err = rediaron.cli.Ping(context.Background()).Result()
	assert.Error(t, err)
}
