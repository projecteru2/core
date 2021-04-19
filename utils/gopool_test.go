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
		create := func(i int) func() {
			return func() {
				time.Sleep(time.Duration(i) * 100 * time.Microsecond)
				cnt++
			}
		}
		pool.Go(context.TODO(), create(i))
	}
	pool.Wait(context.TODO())
	assert.Equal(t, 3, cnt)
}
