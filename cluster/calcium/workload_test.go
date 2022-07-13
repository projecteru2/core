package calcium

import (
	"context"
	"testing"

	storemocks "github.com/projecteru2/core/store/mocks"
	"github.com/projecteru2/core/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestGetWorkload(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()

	ID := "testID"
	workload := &types.Workload{ID: ID}
	store := c.store.(*storemocks.Store)
	store.On("GetWorkload", mock.Anything, mock.Anything).Return(workload, nil)
	_, err := c.GetWorkload(ctx, "")
	assert.Error(t, err)

	_, err = c.GetWorkload(ctx, ID)
	assert.NoError(t, err)

}

func TestGetWorkloads(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()
	ID := "testID"
	workload := &types.Workload{ID: ID}
	workloads := []*types.Workload{workload}

	store := c.store.(*storemocks.Store)
	store.On("GetWorkloads", mock.Anything, mock.Anything).Return(workloads, nil)

	cs, err := c.GetWorkloads(ctx, []string{})
	assert.NoError(t, err)
	assert.Equal(t, len(cs), 1)
	assert.Equal(t, cs[0].ID, ID)
}

func TestListWorkloads(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()
	ID := "testID"
	workloads := []*types.Workload{
		{ID: ID},
	}

	// failed by ListWorkloads
	store := c.store.(*storemocks.Store)
	store.On("ListWorkloads", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, types.ErrNoETCD).Once()
	_, err := c.ListWorkloads(ctx, &types.ListWorkloadsOptions{Appname: "", Entrypoint: "", Nodename: ""})
	assert.Error(t, err)

	// success
	store.On("ListWorkloads", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(workloads, nil)
	cs, err := c.ListWorkloads(ctx, &types.ListWorkloadsOptions{Appname: "", Entrypoint: "", Nodename: ""})
	assert.NoError(t, err)
	assert.Equal(t, len(cs), 1)
	assert.Equal(t, cs[0].ID, ID)
}

func TestListNodeWorkloads(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()
	ID := "testID"
	workloads := []*types.Workload{
		{ID: ID},
	}

	_, err := c.ListNodeWorkloads(ctx, "", nil)
	assert.Error(t, err)

	store := c.store.(*storemocks.Store)
	store.On("ListNodeWorkloads", mock.Anything, mock.Anything, mock.Anything).Return(workloads, nil)
	cs, err := c.ListNodeWorkloads(ctx, "nodename", nil)
	assert.NoError(t, err)
	assert.Equal(t, len(cs), 1)
	assert.Equal(t, cs[0].ID, ID)
}
