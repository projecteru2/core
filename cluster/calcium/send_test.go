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

func TestSend(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()

	_, err := c.Send(ctx, &types.SendOptions{IDs: []string{}, Data: map[string][]byte{"xxx": {}}})
	assert.Error(t, err)
	_, err = c.Send(ctx, &types.SendOptions{IDs: []string{"id"}, Data: map[string][]byte{}})
	assert.Error(t, err)

	tmpfile, err := ioutil.TempFile("", "example")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpfile.Name())
	defer tmpfile.Close()
	opts := &types.SendOptions{
		IDs: []string{"cid"},
		Data: map[string][]byte{
			"/tmp/1": {},
		},
	}
	store := &storemocks.Store{}
	c.store = store
	lock := &lockmocks.DistributedLock{}
	lock.On("Lock", mock.Anything).Return(context.TODO(), nil)
	lock.On("Unlock", mock.Anything).Return(nil)
	store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)
	// failed by GetWorkload
	store.On("GetWorkloads", mock.Anything, mock.Anything).Return(nil, types.ErrNoETCD).Once()
	ch, err := c.Send(ctx, opts)
	assert.NoError(t, err)
	for r := range ch {
		assert.Error(t, r.Error)
	}
	engine := &enginemocks.API{}
	store.On("GetWorkloads", mock.Anything, mock.Anything).Return(
		[]*types.Workload{{ID: "cid", Engine: engine}}, nil,
	)
	// failed by engine
	content, _ := ioutil.ReadAll(tmpfile)
	opts.Data["/tmp/1"] = content
	engine.On("VirtualizationCopyTo",
		mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything,
	).Return(types.ErrCannotGetEngine).Once()
	ch, err = c.Send(ctx, opts)
	assert.NoError(t, err)
	for r := range ch {
		assert.Error(t, r.Error)
	}
	// success
	engine.On("VirtualizationCopyTo",
		mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything,
	).Return(nil)
	ch, err = c.Send(ctx, opts)
	assert.NoError(t, err)
	for r := range ch {
		assert.NoError(t, r.Error)
		assert.Equal(t, r.ID, "cid")
		assert.Equal(t, r.Path, "/tmp/1")
	}
}
