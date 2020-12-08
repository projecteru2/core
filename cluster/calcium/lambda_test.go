package calcium

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	storemocks "github.com/projecteru2/core/store/mocks"
	"github.com/projecteru2/core/types"
)

func TestReapLambdaRecvError(t *testing.T) {
	// TODO
}

func TestReapLambdaRecvData(t *testing.T) {
	noti := make(chan *types.LambdaStatus, 2)
	defer close(noti)

	var watcher <-chan *types.LambdaStatus = noti

	// this will be received by reaper.
	noti <- &types.LambdaStatus{Lambda: &types.Lambda{ID: "1"}}

	store := &storemocks.Store{}
	store.On("WatchLambda", mock.Anything).Return(watcher)

	calcium := NewTestCluster()
	calcium.store = store

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	exit := make(chan struct{})
	wait, err := calcium.ReapLambda(ctx, exit)
	assert.NoError(t, err)

	select {
	case <-ctx.Done():
	case <-wait:
	}

	noti <- &types.LambdaStatus{Lambda: &types.Lambda{ID: "2"}}

	lamb := <-watcher
	assert.Equal(t, "2", lamb.ID)
}

func TestReapLambdaTerminateAsUnreap(t *testing.T) {
	watcher := make(<-chan *types.LambdaStatus)
	exit := make(chan struct{})

	store := &storemocks.Store{}
	store.On("WatchLambda", mock.Anything).Return(watcher)

	calcium := NewTestCluster()
	calcium.store = store

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	wait, err := calcium.ReapLambda(ctx, exit)
	assert.NoError(t, err)

	// Unreap explicitly.
	close(exit)

	select {
	case <-ctx.Done():
		assert.FailNow(t, "waiting for Lambda reaping timeout")
	case <-wait:
	}
}

func TestReapLambdaTerminateAsCtxDone(t *testing.T) {
	watcher := make(<-chan *types.LambdaStatus)

	store := &storemocks.Store{}
	store.On("WatchLambda", mock.Anything).Return(watcher)

	calcium := NewTestCluster()
	calcium.store = store

	ctx, cancel := context.WithCancel(context.Background())
	_, err := calcium.ReapLambda(ctx, make(chan struct{}))
	assert.NoError(t, err)

	// Marks ctx as done.
	cancel()

	select {
	case <-time.After(time.Second):
		assert.FailNow(t, "waiting for Lambda reaping timeout")
	case <-ctx.Done():
	}
}
