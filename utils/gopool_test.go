package utils

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestGoroutinePool(t *testing.T) {
	pool := NewGoroutinePool(1)
	cnt := 0
	for i := 0; i < 3; i++ {
		pool.Go(func() {
			time.Sleep(time.Duration(i) * 100 * time.Microsecond)
			cnt++
		})
	}
	pool.Wait()
	assert.Equal(t, 3, cnt)
}
