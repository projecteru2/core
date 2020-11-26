package rpc

import (
	"testing"

	"context"

	clustermock "github.com/projecteru2/core/cluster/mocks"
	enginemock "github.com/projecteru2/core/engine/mocks"
	enginetypes "github.com/projecteru2/core/engine/types"
	pb "github.com/projecteru2/core/rpc/gen"
	"github.com/projecteru2/core/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func newVibranium() *Vibranium {
	v := &Vibranium{
		cluster: &clustermock.Cluster{},
	}
	return v
}

func TestAddPod(t *testing.T) {
	v := newVibranium()
	opts := &pb.AddPodOptions{}
	cluster := v.cluster.(*clustermock.Cluster)
	cluster.On("AddPod", mock.Anything, mock.Anything, mock.Anything).Return(nil, types.ErrNoETCD).Once()
	_, err := v.AddPod(context.Background(), opts)
	assert.Error(t, err)
	cluster.On("AddPod", mock.Anything, mock.Anything, mock.Anything).Return(&types.Pod{Name: "test", Desc: "test"}, nil)
	_, err = v.AddPod(context.Background(), opts)
	assert.NoError(t, err)
}

func TestAddNode(t *testing.T) {
	v := newVibranium()
	opts := &pb.AddNodeOptions{}
	cluster := v.cluster.(*clustermock.Cluster)
	cluster.On("AddNode", mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything,
	).Return(nil, types.ErrNoETCD).Once()
	_, err := v.AddNode(context.Background(), opts)
	assert.Error(t, err)
	engine := &enginemock.API{}
	node := &types.Node{
		NodeMeta: types.NodeMeta{
			Name: "test",
		},
		Engine: engine,
	}
	engine.On("Info", mock.Anything).Return(&enginetypes.Info{}, nil)
	cluster.On("AddNode", mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything,
	).Return(node, nil)
	_, err = v.AddNode(context.Background(), opts)
	assert.NoError(t, err)
}

func TestSetNodeTranform(t *testing.T) {
	b := &pb.SetNodeOptions{
		Nodename: "a",
		DeltaCpu: map[string]int32{"0": 1, "1": -1},
	}
	o, err := toCoreSetNodeOptions(b)
	assert.Nil(t, err)
	assert.Equal(t, 2, len(o.DeltaCPU))
}
