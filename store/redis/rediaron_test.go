package redis

import (
	"context"
	"errors"
	"fmt"
	"os"
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

// realRedisEnv names a running redis the tests use instead of miniredis; the CI redis job sets it.
const realRedisEnv = "ERU_TEST_REDIS_ADDR"

func TestRediaron(t *testing.T) {
	cli, s := testRedis(t)
	config := types.Config{}
	config.LockTimeout = 10 * time.Second
	config.GlobalTimeout = 30 * time.Second
	config.MaxConcurrency = 100000

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	factory.InitEngineCache(ctx, config, nil)

	pool, _ := utils.NewPool(20)
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
	s.False(s.exists("a"))
	value := s.get("b")
	s.Equal("old", value)
}

func (s *RediaronTestSuite) TestCreateAndDecrNeedsTheCounter() {
	ctx := s.T().Context()
	s.ErrorIs(s.rediaron.CreateAndDecr(ctx, map[string]string{"a": "1"}, "counter"), types.ErrKeyNotExists)
	s.False(s.exists("a"))

	s.NoError(s.rediaron.cli.Set(ctx, "counter", "2", 0).Err())
	s.NoError(s.rediaron.CreateAndDecr(ctx, map[string]string{"a": "1"}, "counter"))
	counter := s.get("counter")
	s.Equal("1", counter)

	s.ErrorIs(s.rediaron.CreateAndDecr(ctx, map[string]string{"a": "1", "c": "3"}, "counter"), ErrAlreadyExists)
	counter = s.get("counter")
	s.Equal("1", counter)
	s.False(s.exists("c"))
}

func (s *RediaronTestSuite) TestBindStatusRefreshesAnUnchangedValueInPlace() {
	ctx := s.T().Context()
	s.NoError(s.rediaron.cli.Set(ctx, "entity", "1", 0).Err())

	s.NoError(s.rediaron.BindStatus(ctx, "entity", "status", "v1", 10))
	s.InDelta(10, s.ttl("status").Seconds(), 1)
	s.advance(4 * time.Second)
	s.NoError(s.rediaron.BindStatus(ctx, "entity", "status", "v1", 10))
	s.InDelta(10, s.ttl("status").Seconds(), 1)

	s.NoError(s.rediaron.BindStatus(ctx, "entity", "status", "v2", 10))
	value := s.get("status")
	s.Equal("v2", value)

	s.NoError(s.rediaron.BindStatus(ctx, "entity", "status", "v2", 0))
	s.Zero(s.ttl("status"))
	s.ErrorIs(s.rediaron.BindStatus(ctx, "absent", "orphan", "v2", 10), types.ErrInvaildCount)
	s.False(s.exists("orphan"))
	s.NoError(s.rediaron.BindStatus(ctx, "absent", "orphan", "v2", 0))
	s.InDelta(time.Hour.Seconds(), s.ttl("orphan").Seconds(), 1)

	s.NoError(s.rediaron.BindStatus(ctx, "entity", "status", "v2", 10))
	s.advance(11 * time.Second)
	s.False(s.exists("status"))
	s.NoError(s.rediaron.BindStatus(ctx, "entity", "status", "v2", 10))
	s.InDelta(10, s.ttl("status").Seconds(), 1)
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

func (s *RediaronTestSuite) exists(key string) bool {
	n, err := s.rediaron.cli.Exists(s.T().Context(), key).Result()
	s.Require().NoError(err)
	return n > 0
}

func (s *RediaronTestSuite) get(key string) string {
	value, err := s.rediaron.cli.Get(s.T().Context(), key).Result()
	if errors.Is(err, redis.Nil) {
		return ""
	}
	s.Require().NoError(err)
	return value
}

func (s *RediaronTestSuite) ttl(key string) time.Duration {
	ttl, err := s.rediaron.cli.TTL(s.T().Context(), key).Result()
	s.Require().NoError(err)
	return max(ttl, 0)
}

func (s *RediaronTestSuite) advance(d time.Duration) {
	if s.rediserver != nil {
		s.rediserver.FastForward(d)
		return
	}
	time.Sleep(d + 500*time.Millisecond)
}

func triggerMockedKeyspaceNotification(cli *redis.Client, key, action string) {
	channel := fmt.Sprintf(keyNotifyPrefix, 0, key)
	cli.Publish(context.Background(), channel, action).Result()
}

func testRedis(t testing.TB) (*redis.Client, *miniredis.Miniredis) {
	t.Helper()
	if addr := os.Getenv(realRedisEnv); addr != "" {
		cli := redis.NewClient(&redis.Options{Addr: addr})
		t.Cleanup(func() { _ = cli.Close() })
		return cli, nil
	}
	s := miniredis.RunT(t)
	cli := redis.NewClient(&redis.Options{Addr: s.Addr()})
	t.Cleanup(func() { _ = cli.Close() })
	return cli, s
}
