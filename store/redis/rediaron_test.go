package redis

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/suite"

	"github.com/projecteru2/core/engine/factory"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

func TestRediaron(t *testing.T) {
	s, err := miniredis.Run()
	if err != nil {
		t.Fail()
	}
	defer s.Close()

	config := types.Config{}
	config.LockTimeout = 10 * time.Second
	config.GlobalTimeout = 30 * time.Second
	config.MaxConcurrency = 100000

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	factory.InitEngineCache(ctx, config, nil)

	cli := redis.NewClient(&redis.Options{
		Addr: s.Addr(),
		DB:   0,
	})

	pool, _ := utils.NewPool(20)

	defer cli.Close()
	suite.Run(t, &RediaronTestSuite{
		rediserver: s,
		rediaron:   newRediaron(cli, config, pool),
	})
}

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

func (s *RediaronTestSuite) TestGetMultiWrapsTheMissingKeysOwnError() {
	ctx := context.Background()
	s.NoError(s.rediaron.cli.HSet(ctx, "hash", "f", "v").Err())

	_, err := s.rediaron.GetMulti(ctx, []string{"hash", "absent"})
	s.Error(err)
	s.True(s.rediaron.NotFound(err))
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

func (s *RediaronTestSuite) TestKeyNotifyCancellationUnblocksPendingMessage() {
	ctx, cancel := context.WithCancel(context.Background())
	ch := s.rediaron.KNotify(ctx, "a*")
	s.Require().Eventually(func() bool {
		count, err := s.rediaron.cli.PubSubNumPat(context.Background()).Result()
		return err == nil && count > 0
	}, time.Second, 10*time.Millisecond)

	triggerMockedKeyspaceNotification(s.rediaron.cli, "aaa", actionSet)
	time.Sleep(50 * time.Millisecond)
	cancel()
	s.Require().Eventually(func() bool {
		count, err := s.rediaron.cli.PubSubNumPat(context.Background()).Result()
		return err == nil && count == 0
	}, time.Second, 10*time.Millisecond)

	select {
	case _, ok := <-ch:
		s.False(ok)
	case <-time.After(time.Second):
		s.FailNow("key notification did not stop")
	}
}

func triggerMockedKeyspaceNotification(cli *redis.Client, key, action string) {
	channel := fmt.Sprintf(keyNotifyPrefix, 0, key)
	cli.Publish(context.Background(), channel, action).Result()
}
