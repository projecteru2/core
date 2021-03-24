package redis

import (
	"context"
	"time"
)

func (s *RediaronTestSuite) TestCreateLock() {
	ctx := context.Background()

	lock, err := s.rediaron.CreateLock("test", time.Second)
	s.NoError(err)
	s.NotNil(lock)

	_, err = lock.Lock(ctx)
	s.NoError(err)

	err = lock.Unlock(ctx)
	s.NoError(err)
}
