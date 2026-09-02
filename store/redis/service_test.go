package redis

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/projecteru2/core/store/common"
)

func (s *RediaronTestSuite) TestRegisterServiceWithDeregister() {
	m := s.rediaron
	ctx := s.T().Context()
	svc := "svc"
	path := fmt.Sprintf(common.ServiceStatusKey, svc)
	_, deregister, err := m.RegisterService(ctx, svc, time.Minute)
	s.NoError(err)

	v, err := m.GetOne(ctx, path)
	s.NoError(err)
	s.NotEmpty(v)

	deregister()
	v, err = m.GetOne(ctx, path)
	s.Error(err)
	s.Empty(v)
}

func (s *RediaronTestSuite) TestServiceStatusStream() {
	m := s.rediaron
	ctx, cancel := context.WithCancel(s.T().Context())

	go func() {
		time.Sleep(3 * time.Second)
		cancel()
	}()

	_, unregisterService1, err := m.RegisterService(ctx, "127.0.0.1:5001", time.Second)
	s.NoError(err)

	ch, err := m.ServiceStatusStream(ctx)
	s.NoError(err)

	s.Equal(<-ch, []string{"127.0.0.1:5001"})

	_, _, err = m.RegisterService(ctx, "127.0.0.1:5002", time.Second)
	s.NoError(err)
	time.Sleep(500 * time.Millisecond)
	triggerMockedKeyspaceNotification(s.rediaron.cli, fmt.Sprintf(common.ServiceStatusKey, "127.0.0.1:5002"), actionSet)

	endpoints := <-ch
	sort.Strings(endpoints)
	s.Equal(endpoints, []string{"127.0.0.1:5001", "127.0.0.1:5002"})

	unregisterService1()
	time.Sleep(500 * time.Millisecond)
	triggerMockedKeyspaceNotification(s.rediaron.cli, fmt.Sprintf(common.ServiceStatusKey, "127.0.0.1:5001"), actionDel)

	s.advance(time.Second)
	s.Equal(<-ch, []string{"127.0.0.1:5002"})
}

func (s *RediaronTestSuite) TestServiceStatusStreamResnapshotsAfterReconnect() {
	if s.rediserver == nil {
		s.T().Skip("restarts the server, which only miniredis can do")
	}
	ctx, cancel := context.WithCancel(s.T().Context())
	defer cancel()
	key := fmt.Sprintf(common.ServiceStatusKey, "127.0.0.1:5001")
	s.Require().NoError(s.rediaron.cli.Set(ctx, key, "token", 0).Err())

	ch, err := s.rediaron.ServiceStatusStream(ctx)
	s.Require().NoError(err)
	select {
	case endpoints := <-ch:
		s.Equal([]string{"127.0.0.1:5001"}, endpoints)
	case <-time.After(time.Second):
		s.FailNow("service status stream did not start")
	}

	s.Require().Eventually(func() bool {
		count, err := s.rediaron.cli.PubSubNumPat(ctx).Result()
		return err == nil && count > 0
	}, time.Second, 10*time.Millisecond)
	s.rediserver.Close()
	s.True(s.rediserver.Del(key))
	s.Require().NoError(s.rediserver.Restart())

	select {
	case endpoints := <-ch:
		s.Empty(endpoints)
	case <-time.After(5 * time.Second):
		s.FailNow("service status stream did not resnapshot")
	}
}
