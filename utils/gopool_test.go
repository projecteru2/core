package utils

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewPool(t *testing.T) {
	pool, err := NewPool(20)
	assert.NoError(t, err)
	assert.Equal(t, pool.Cap(), 20)
}

func TestPoolQueuesTasksInsteadOfDroppingThem(t *testing.T) {
	pool, err := NewPool(1)
	assert.NoError(t, err)

	occupied, release, ran := make(chan struct{}), make(chan struct{}), make(chan struct{})
	releaseOnce := sync.OnceFunc(func() { close(release) })
	defer releaseOnce()
	assert.NoError(t, pool.Invoke(func() { close(occupied); <-release }))
	<-occupied

	invoked := make(chan error, 1)
	go func() { invoked <- pool.Invoke(func() { close(ran) }) }()

	select {
	case err := <-invoked:
		t.Fatalf("a saturated pool rejected the task instead of queueing it: %v", err)
	case <-time.After(100 * time.Millisecond):
	}

	releaseOnce()
	assert.NoError(t, <-invoked)
	select {
	case <-ran:
	case <-time.After(5 * time.Second):
		t.Fatal("the queued task never ran on a saturated pool")
	}
}
