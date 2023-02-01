package calcium

import (
	"context"
	"testing"

	enginemocks "github.com/projecteru2/core/engine/mocks"
	enginetypes "github.com/projecteru2/core/engine/types"
	lockmocks "github.com/projecteru2/core/lock/mocks"
	"github.com/projecteru2/core/log"
	resourcemocks "github.com/projecteru2/core/resource/mocks"
	resourcetypes "github.com/projecteru2/core/resource/types"
	storemocks "github.com/projecteru2/core/store/mocks"
	"github.com/projecteru2/core/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestRemapResource(t *testing.T) {
	c := NewTestCluster()
	store := c.store.(*storemocks.Store)
	rmgr := c.rmgr.(*resourcemocks.Manager)
	rmgr.On("GetNodeResourceInfo", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(
		resourcetypes.Resources{"test": {"abc": 123}},
		resourcetypes.Resources{"test": {"abc": 123}},
		[]string{types.ErrMockError.Error()},
		nil)
	rmgr.On("Remap", mock.Anything, mock.Anything, mock.Anything).Return(
		resourcetypes.Resources{},
		nil,
	)
	engine := &enginemocks.API{}
	node := &types.Node{Engine: engine}

	workload := &types.Workload{
		Resources: resourcetypes.Resources{},
	}
	store.On("ListNodeWorkloads", mock.Anything, mock.Anything, mock.Anything).Return([]*types.Workload{workload}, nil)
	ch := make(chan enginetypes.VirtualizationRemapMessage, 1)
	ch <- enginetypes.VirtualizationRemapMessage{}
	close(ch)
	engine.On("VirtualizationResourceRemap", mock.Anything, mock.Anything).Return((<-chan enginetypes.VirtualizationRemapMessage)(ch), nil)
	_, err := c.doRemapResource(context.Background(), node)
	assert.Nil(t, err)

	store.On("GetNode", mock.Anything, mock.Anything).Return(node, nil)
	lock := &lockmocks.DistributedLock{}
	lock.On("Lock", mock.Anything).Return(context.Background(), nil)
	lock.On("Unlock", mock.Anything).Return(nil)
	store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)
	c.RemapResourceAndLog(context.Background(), log.WithField("test", "zc"), node)
}
