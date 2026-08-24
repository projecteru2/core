package utils

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestPollReachabilityReleasesTheLockWhenAProbeFails(t *testing.T) {
	restore := reachabilityInterval
	reachabilityInterval = time.Millisecond
	t.Cleanup(func() { reachabilityInterval = restore })

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	p := NewEndpointPusher()
	go p.pollReachability(ctx, "nonexistent.invalid:1")

	time.Sleep(100 * time.Millisecond)
	assert.True(t, p.TryLock(), "the lock is still held after a failed probe")
	p.Unlock()
}
