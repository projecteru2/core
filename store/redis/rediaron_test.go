package redis

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/projecteru2/core/engine/factory"
	"github.com/projecteru2/core/types"
)

type RediaronTestSuite struct {
	suite.Suite

	rediaron   *Rediaron
	rediserver *miniredis.Miniredis
}

func (s *RediaronTestSuite) SetupTest() {
	s.rediaron.cli.FlushAll(context.Background())
}

func (s *RediaronTestSuite) TearDownTest() {
	s.rediaron.cli.FlushAll(context.Background())
}

func (s *RediaronTestSuite) TestIsRedisNoKeyError() {
	_, err := s.rediaron.cli.Get(context.Background(), "thiskeydoesnotexistsofcourseitdoesnt").Result()
	s.True(isRedisNoKeyError(err))

	s.rediaron.cli.Set(context.Background(), "key1", "value1", 0)
	_, err = s.rediaron.cli.Get(context.Background(), "key1").Result()
	s.False(isRedisNoKeyError(err))

	s.False(isRedisNoKeyError(fmt.Errorf("i am not redis no key error")))
}

func (s *RediaronTestSuite) TestKeyNotify() {
	ctx, cancel := context.WithCancel(context.Background())
	ch := s.rediaron.KNotify(ctx, "a*")
	go func() {
		time.Sleep(2 * time.Second)
		cancel()
	}()

	// trigger manually
	time.Sleep(time.Second)
	s.rediaron.cli.Set(context.Background(), "aaa", 1, 0)
	triggerMockedKeyspaceNotification(s.rediaron.cli, "aaa", actionSet)
	s.rediaron.cli.Set(context.Background(), "aab", 1, 0)
	triggerMockedKeyspaceNotification(s.rediaron.cli, "aab", actionSet)
	s.rediaron.cli.Set(context.Background(), "bab", 1, 0)
	triggerMockedKeyspaceNotification(s.rediaron.cli, "bab", actionSet)
	s.rediaron.cli.Del(context.Background(), "aaa")
	triggerMockedKeyspaceNotification(s.rediaron.cli, "aaa", actionDel)

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

func TestRediaron(t *testing.T) {
	s, err := miniredis.Run()
	if err != nil {
		t.Fail()
	}
	defer s.Close()

	config := types.Config{}
	config.LockTimeout = 10 * time.Second
	config.GlobalTimeout = 30 * time.Second
	config.MaxConcurrency = 20

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	factory.InitEngineCache(ctx, config)

	cli := redis.NewClient(&redis.Options{
		Addr: s.Addr(),
		DB:   0,
	})
	defer cli.Close()
	suite.Run(t, &RediaronTestSuite{
		rediserver: s,
		rediaron: &Rediaron{
			cli:    cli,
			config: config,
		},
	})
}

func TestTerminateEmbeddedStorage(t *testing.T) {
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

	rediaron := &Rediaron{
		cli: cli,
	}

	_, err = rediaron.cli.Ping(context.Background()).Result()
	assert.NoError(t, err)

	rediaron.TerminateEmbededStorage()
	_, err = rediaron.cli.Ping(context.Background()).Result()
	assert.Error(t, err)
}

func triggerMockedKeyspaceNotification(cli *redis.Client, key, action string) {
	channel := fmt.Sprintf(keyNotifyPrefix, 0, key)
	cli.Publish(context.Background(), channel, action).Result()
}
