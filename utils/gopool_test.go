package utils

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestGoroutinePool(t *testing.T) {
	pool := NewGoroutinePool(1)
	cnt := 0
	for i := 0; i < 3; i++ {
		pool.Go(context.TODO(), func() {
			time.Sleep(time.Duration(i) * 100 * time.Microsecond)
			cnt++
		})
	}
	pool.Wait(context.TODO())
	assert.Equal(t, 3, cnt)
}
