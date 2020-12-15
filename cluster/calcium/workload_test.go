package calcium

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	storemocks "github.com/projecteru2/core/store/mocks"
	"github.com/projecteru2/core/types"
)

func TestListWorkloads(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()
	ID := "testID"
	workloads := []*types.Workload{
		{ID: ID},
	}

	store := &storemocks.Store{}
	c.store = store
	store.On("ListWorkloads", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(workloads, nil)
	store.On("ListNodeWorkloads", mock.Anything, mock.Anything, mock.Anything).Return(workloads, nil)

	cs, err := c.ListWorkloads(ctx, &types.ListWorkloadsOptions{Appname: "", Entrypoint: "", Nodename: ""})
	assert.NoError(t, err)
	assert.Equal(t, len(cs), 1)
	assert.Equal(t, cs[0].ID, ID)

	_, err = c.ListNodeWorkloads(ctx, "", nil)
	assert.Error(t, err)

	cs, err = c.ListNodeWorkloads(ctx, "nodename", nil)
	assert.NoError(t, err)
	assert.Equal(t, len(cs), 1)
	assert.Equal(t, cs[0].ID, ID)
}

func TestGetWorkloads(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()
	ID := "testID"
	workload := &types.Workload{ID: ID}
	workloads := []*types.Workload{workload}

	store := &storemocks.Store{}
	c.store = store
	store.On("GetWorkload", mock.Anything, mock.Anything).Return(workload, nil)
	store.On("GetWorkloads", mock.Anything, mock.Anything).Return(workloads, nil)

	_, err := c.GetWorkload(ctx, "")
	assert.Error(t, err)

	savedWorkload, err := c.GetWorkload(ctx, "someid")
	assert.NoError(t, err)
	assert.Equal(t, savedWorkload.ID, ID)
	cs, err := c.GetWorkloads(ctx, []string{})
	assert.NoError(t, err)
	assert.Equal(t, len(cs), 1)
	assert.Equal(t, cs[0].ID, ID)
}
