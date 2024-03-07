package calcium

import (
	"context"
	"io/ioutil"
	"os"
	"testing"

	enginemocks "github.com/projecteru2/core/engine/mocks"
	lockmocks "github.com/projecteru2/core/lock/mocks"
	storemocks "github.com/projecteru2/core/store/mocks"
	"github.com/projecteru2/core/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestSendLarge(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()

	tmpfile, err := os.CreateTemp("", "example")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpfile.Name())
	defer tmpfile.Close()
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
	lock.On("Lock", mock.Anything).Return(context.TODO(), nil)
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
	engine := &enginemocks.API{}
	store.On("GetWorkloads", mock.Anything, mock.Anything).Return(
		[]*types.Workload{{ID: "cid", Engine: engine}}, nil,
	)
	// failed by engine
	content, _ := ioutil.ReadAll(tmpfile)
	opts.Chunk = content
	engine.On("VirtualizationCopyChunkTo",
		mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything,
	).Return(types.ErrMockError).Once()
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
	// success
	engine.On("VirtualizationCopyChunkTo",
		mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything,
	).Return(nil)
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
