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
	"github.com/projecteru2/core/store/common"
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

	ctx, cancel := context.WithCancel(t.Context())
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
	s.rediaron.cli.FlushAll(s.T().Context())
}

func (s *RediaronTestSuite) TearDownTest() {
	s.rediaron.cli.FlushAll(s.T().Context())
}

func (s *RediaronTestSuite) TestIsRedisNoKeyError() {
	_, err := s.rediaron.cli.Get(s.T().Context(), "thiskeydoesnotexistsofcourseitdoesnt").Result()
	s.True(isRedisNoKeyError(err))

	s.rediaron.cli.Set(s.T().Context(), "key1", "value1", 0)
	_, err = s.rediaron.cli.Get(s.T().Context(), "key1").Result()
	s.False(isRedisNoKeyError(err))

	s.False(isRedisNoKeyError(fmt.Errorf("i am not redis no key error")))
}

func (s *RediaronTestSuite) TestGetMultiWrapsTheMissingKeysOwnError() {
	ctx := s.T().Context()
	s.NoError(s.rediaron.cli.HSet(ctx, "hash", "f", "v").Err())

	_, err := s.rediaron.GetMulti(ctx, []string{"hash", "absent"})
	s.Error(err)
	s.True(s.rediaron.NotFound(err))
}

func (s *RediaronTestSuite) TestKeyNotify() {
	ctx, cancel := context.WithCancel(s.T().Context())
	ch := s.rediaron.KNotify(ctx, "a*")
	go func() {
		time.Sleep(2 * time.Second)
		cancel()
	}()

	time.Sleep(time.Second)
	s.rediaron.cli.Set(s.T().Context(), "aaa", 1, 0)
	triggerMockedKeyspaceNotification(s.rediaron.cli, "aaa", actionSet)
	s.rediaron.cli.Set(s.T().Context(), "aab", 1, 0)
	triggerMockedKeyspaceNotification(s.rediaron.cli, "aab", actionSet)
	s.rediaron.cli.Set(s.T().Context(), "bab", 1, 0)
	triggerMockedKeyspaceNotification(s.rediaron.cli, "bab", actionSet)
	s.rediaron.cli.Del(s.T().Context(), "aaa")
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
	ctx, cancel := context.WithCancel(s.T().Context())
	ch := s.rediaron.KNotify(ctx, "a*")
	s.Require().Eventually(func() bool {
		count, err := s.rediaron.cli.PubSubNumPat(s.T().Context()).Result()
		return err == nil && count > 0
	}, time.Second, 10*time.Millisecond)

	triggerMockedKeyspaceNotification(s.rediaron.cli, "aaa", actionSet)
	time.Sleep(50 * time.Millisecond)
	cancel()
	s.Require().Eventually(func() bool {
		count, err := s.rediaron.cli.PubSubNumPat(s.T().Context()).Result()
		return err == nil && count == 0
	}, time.Second, 10*time.Millisecond)

	select {
	case _, ok := <-ch:
		s.False(ok)
	case <-time.After(time.Second):
		s.FailNow("key notification did not stop")
	}
}

func (s *RediaronTestSuite) TestCreateWritesNothingOnAConflict() {
	ctx := s.T().Context()
	s.NoError(s.rediaron.cli.Set(ctx, "b", "old", 0).Err())

	s.ErrorIs(s.rediaron.Create(ctx, map[string]string{"a": "1", "b": "2"}), ErrAlreadyExists)
	s.False(s.rediserver.Exists("a"))
	value, _ := s.rediserver.Get("b")
	s.Equal("old", value)
}

func (s *RediaronTestSuite) TestCreateAndDecrNeedsTheCounter() {
	ctx := s.T().Context()
	s.ErrorIs(s.rediaron.CreateAndDecr(ctx, map[string]string{"a": "1"}, "counter"), ErrKeyNotExists)
	s.False(s.rediserver.Exists("a"))

	s.NoError(s.rediaron.cli.Set(ctx, "counter", "2", 0).Err())
	s.NoError(s.rediaron.CreateAndDecr(ctx, map[string]string{"a": "1"}, "counter"))
	counter, _ := s.rediserver.Get("counter")
	s.Equal("1", counter)

	s.ErrorIs(s.rediaron.CreateAndDecr(ctx, map[string]string{"a": "1", "c": "3"}, "counter"), ErrAlreadyExists)
	counter, _ = s.rediserver.Get("counter")
	s.Equal("1", counter)
	s.False(s.rediserver.Exists("c"))
}

func (s *RediaronTestSuite) TestBindStatusRefreshesAnUnchangedValueInPlace() {
	ctx := s.T().Context()
	s.NoError(s.rediaron.cli.Set(ctx, "entity", "1", 0).Err())

	s.NoError(s.rediaron.BindStatus(ctx, "entity", "status", "v1", 10))
	s.Equal(10*time.Second, s.rediserver.TTL("status"))
	s.rediserver.FastForward(4 * time.Second)
	s.NoError(s.rediaron.BindStatus(ctx, "entity", "status", "v1", 10))
	s.Equal(10*time.Second, s.rediserver.TTL("status"))

	s.NoError(s.rediaron.BindStatus(ctx, "entity", "status", "v2", 10))
	value, _ := s.rediserver.Get("status")
	s.Equal("v2", value)

	s.NoError(s.rediaron.BindStatus(ctx, "entity", "status", "v2", 0))
	s.Zero(s.rediserver.TTL("status"))
	s.ErrorIs(s.rediaron.BindStatus(ctx, "absent", "status", "v2", 0), types.ErrInvaildCount)

	s.NoError(s.rediaron.BindStatus(ctx, "entity", "status", "v2", 10))
	s.rediserver.FastForward(11 * time.Second)
	s.False(s.rediserver.Exists("status"))
	s.NoError(s.rediaron.BindStatus(ctx, "entity", "status", "v2", 10))
	s.Equal(10*time.Second, s.rediserver.TTL("status"))
}

func (s *RediaronTestSuite) TestPrefixReadsTakeGlobMetacharactersLiterally() {
	ctx := s.T().Context()
	s.NoError(s.rediaron.cli.MSet(ctx, "node/web-[01]/x", "1", "node/web-0/x", "2", "node/web-1/x", "3").Err())

	keys, err := s.rediaron.ListPrefix(ctx, "node/web-[01]/")
	s.NoError(err)
	s.Equal([]string{"node/web-[01]/x"}, keys)

	data, err := s.rediaron.GetPrefix(ctx, "node/web-", 0)
	s.NoError(err)
	s.Len(data, 3)
}

func (s *RediaronTestSuite) TestWatchSkipsTTLRefreshes() {
	ctx, cancel := context.WithCancel(s.T().Context())
	events := []common.Event{}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for event := range s.rediaron.Watch(ctx, "a") {
			events = append(events, event)
		}
	}()
	s.Require().Eventually(func() bool {
		count, err := s.rediaron.cli.PubSubNumPat(s.T().Context()).Result()
		return err == nil && count > 0
	}, time.Second, 10*time.Millisecond)

	triggerMockedKeyspaceNotification(s.rediaron.cli, "aaa", "expire")
	triggerMockedKeyspaceNotification(s.rediaron.cli, "aab", actionSet)
	triggerMockedKeyspaceNotification(s.rediaron.cli, "aaa", actionExpired)
	time.Sleep(50 * time.Millisecond)
	cancel()
	<-done

	s.Equal([]common.Event{{Key: "aab", Type: common.EventPut}, {Key: "aaa", Type: common.EventExpire}}, events)
}

func triggerMockedKeyspaceNotification(cli *redis.Client, key, action string) {
	channel := fmt.Sprintf(keyNotifyPrefix, 0, key)
	cli.Publish(context.Background(), channel, action).Result()
}
