package calcium

import (
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	enginemocks "github.com/projecteru2/core/engine/mocks"
	lockmocks "github.com/projecteru2/core/lock/mocks"
	storemocks "github.com/projecteru2/core/store/mocks"
	"github.com/projecteru2/core/types"
)

func TestSendLarge(t *testing.T) {
	c := NewTestCluster()
	ctx := t.Context()

	opts := &types.SendLargeFileOptions{
		IDs:   []string{"cid"},
		Size:  1,
		Dst:   "/tmp/1",
		Chunk: []byte{},
	}
	optsChan := make(chan *types.SendLargeFileOptions)
	store := &storemocks.Store{}
	c.store = store
	lock := &lockmocks.DistributedLock{}
	lock.On("Lock", mock.Anything).Return(context.Background(), nil)
	lock.On("Unlock", mock.Anything).Return(nil)
	store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)
	store.On("GetWorkloads", mock.Anything, mock.Anything).Return(nil, types.ErrMockError).Once()
	ch := c.SendLargeFile(ctx, optsChan)
	go func() {
		optsChan <- opts
		close(optsChan)
	}()
	for r := range ch {
		assert.Error(t, r.Error)
	}
	engine := &chunkEngine{API: &enginemocks.API{}}
	store.On("GetWorkloads", mock.Anything, mock.Anything).Return(
		[]*types.Workload{{ID: "cid", Engine: engine}}, nil,
	)
	engine.err = types.ErrMockError
	optsChan = make(chan *types.SendLargeFileOptions)
	ch = c.SendLargeFile(ctx, optsChan)
	go func() {
		optsChan <- opts
		close(optsChan)
	}()
	for r := range ch {
		t.Log(r.Error)
		assert.Error(t, r.Error)
	}
	engine.err = nil
	optsChan = make(chan *types.SendLargeFileOptions)
	ch = c.SendLargeFile(ctx, optsChan)
	go func() {
		optsChan <- opts
		close(optsChan)
	}()
	for r := range ch {
		assert.Equal(t, r.ID, "cid")
		assert.Equal(t, r.Path, "/tmp/1")
		assert.NoError(t, r.Error)
	}
}

type chunkEngine struct {
	*enginemocks.API
	err error
}

func (e *chunkEngine) VirtualizationCopyChunkTo(_ context.Context, _, _ string, _ int64, content io.Reader, _, _ int, _ int64) error {
	_, _ = io.Copy(io.Discard, content)
	return e.err
}
