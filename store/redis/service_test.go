package redis

import (
	"context"
	"fmt"
	"sort"
	"time"
)

func (s *RediaronTestSuite) TestRegisterServiceWithDeregister() {
	m := s.rediaron
	ctx := context.Background()
	svc := "svc"
	path := fmt.Sprintf(serviceStatusKey, svc)
	_, deregister, err := m.RegisterService(ctx, svc, time.Minute)
	s.NoError(err)

	v, err := m.GetOne(ctx, path)
	s.NoError(err)
	s.Equal(ephemeralValue, v)

	deregister()
	//time.Sleep(time.Second)
	v, err = m.GetOne(ctx, path)
	s.Error(err)
	s.Empty(v)
}

func (s *RediaronTestSuite) TestServiceStatusStream() {
	m := s.rediaron
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		time.Sleep(3 * time.Second)
		cancel()
	}()

	_, unregisterService1, err := m.RegisterService(ctx, "127.0.0.1:5001", time.Second)
	s.NoError(err)

	ch, err := m.ServiceStatusStream(ctx)
	s.NoError(err)

	s.Equal(<-ch, []string{"127.0.0.1:5001"})

	// register and triggers set manually
	_, _, err = m.RegisterService(ctx, "127.0.0.1:5002", time.Second)
	s.NoError(err)
	time.Sleep(500 * time.Millisecond)
	triggerMockedKeyspaceNotification(s.rediaron.cli, fmt.Sprintf(serviceStatusKey, "127.0.0.1:5002"), actionSet)

	endpoints := <-ch
	sort.Strings(endpoints)
	s.Equal(endpoints, []string{"127.0.0.1:5001", "127.0.0.1:5002"})

	// unregister and triggers del manually
	_, _, err = m.RegisterService(ctx, "127.0.0.1:5002", time.Second)
	unregisterService1()
	time.Sleep(500 * time.Millisecond)
	triggerMockedKeyspaceNotification(s.rediaron.cli, fmt.Sprintf(serviceStatusKey, "127.0.0.1:5001"), actionDel)

	// trigger fastforward to clean the value
	s.rediserver.FastForward(time.Second)
	s.Equal(<-ch, []string{"127.0.0.1:5002"})
}
