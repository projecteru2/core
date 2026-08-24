package utils

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestPushReachesEveryRegisteredChannel(t *testing.T) {
	p := &EndpointPusher{}
	first, second := make(chan []string), make(chan []string)
	p.Register(first)
	p.Register(second)

	endpoints := []string{"127.0.0.1:5001", "127.0.0.1:5002"}
	go p.Push(t.Context(), endpoints)

	assert.Equal(t, endpoints, <-first)
	assert.Equal(t, endpoints, <-second)
}

func TestPushStopsWhenTheContextIsDone(t *testing.T) {
	p := &EndpointPusher{}
	p.Register(make(chan []string))

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		p.Push(ctx, []string{"127.0.0.1:5001"})
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("push blocked on an unread channel after the context was done")
	}
}
