package etcdv3

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/projecteru2/core/types"
)

func TestWatchLambdaRecvDataError(t *testing.T) {
	mercury := NewMercury(t)
	defer mercury.TerminateEmbededStorage()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	watcher := mercury.WatchLambda(ctx)

	// Inserts invalid data
	id := "id"
	data := map[string]string{makeLambdaKey(id): "x"}
	_, err := mercury.batchCreate(context.Background(), data)
	assert.NoError(t, err)

	select {
	case <-ctx.Done():
		assert.FailNow(t, "watch Lambda timeout")
	case lamb := <-watcher:
		assert.NotNil(t, lamb)
		assert.Error(t, lamb.Error)
	}
}

func TestWatchLambdaRecvCtxDone(t *testing.T) {
	mercury := NewMercury(t)
	defer mercury.TerminateEmbededStorage()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	cancel() // Marks the ctx as done.

	select {
	case <-mercury.WatchLambda(ctx):
		assert.FailNow(t, "unexpected watching")
	case <-ctx.Done():
	}
}

func TestGetLambda(t *testing.T) {
	mercury := NewMercury(t)
	defer mercury.TerminateEmbededStorage()

	exp := &types.Lambda{
		ID: "id",
	}
	assert.NoError(t, mercury.AddLambda(context.Background(), exp))

	real, err := mercury.GetLambda(context.Background(), exp.ID)
	assert.NoError(t, err)
	assert.NotNil(t, real)
}

func TestWatchLambda(t *testing.T) {
	mercury := NewMercury(t)
	defer mercury.TerminateEmbededStorage()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	watcher := mercury.WatchLambda(ctx)

	exp := &types.Lambda{
		ID: "id",
	}
	// Add a new one to trigger activating watcher.
	assert.NoError(t, mercury.AddLambda(context.Background(), exp))

	select {
	case <-ctx.Done():
		assert.FailNow(t, "watch Lambda timeout")
	case lamb := <-watcher:
		assert.NotNil(t, lamb)
		assert.NoError(t, lamb.Error)
		assert.Equal(t, exp.ID, lamb.ID)
	}
}

func TestAddLambdaShouldGenerateID(t *testing.T) {
	mercury := NewMercury(t)
	defer mercury.TerminateEmbededStorage()

	exp := &types.Lambda{}
	assert.True(t, len(exp.ID) == 0)
	assert.NoError(t, mercury.AddLambda(context.Background(), exp))
	assert.True(t, len(exp.ID) > 0)
	assert.True(t, strings.Index(exp.ID, "-") == -1)
}

func TestAddLambdaWithID(t *testing.T) {
	mercury := NewMercury(t)
	defer mercury.TerminateEmbededStorage()

	exp := &types.Lambda{
		ID: "1",
	}
	assert.NoError(t, mercury.AddLambda(context.Background(), exp))

	act, err := mercury.GetLambda(context.Background(), exp.ID)
	assert.NoError(t, err)
	assert.NotNil(t, act)
	assert.Equal(t, exp.ID, act.ID)
}

func TestGetLambdasFailedAsNoSuchPartialOfThem(t *testing.T) {
	mercury := NewMercury(t)
	defer mercury.TerminateEmbededStorage()

	exp := &types.Lambda{
		ID: "1",
	}
	assert.NoError(t, mercury.AddLambda(context.Background(), exp))

	lambdas, err := mercury.GetLambdas(context.Background(), []string{"1", "2"})
	assert.NotNil(t, err)
	assert.Nil(t, lambdas)
}

func TestGetLambdasFailedAsInvalidJSON(t *testing.T) {
	mercury := NewMercury(t)
	defer mercury.TerminateEmbededStorage()

	id := "id"
	key := makeLambdaKey(id)

	// Inserts invalid data
	data := map[string]string{key: "x"}
	_, err := mercury.batchCreate(context.Background(), data)
	assert.NoError(t, err)

	lambda, err := mercury.GetLambda(context.Background(), id)
	assert.NotNil(t, err)
	assert.Nil(t, lambda)
}
