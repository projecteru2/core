package calcium

import (
	"fmt"
	"testing"

	"github.com/projecteru2/core/network/calico"
	"github.com/projecteru2/core/scheduler/simple"
	"github.com/projecteru2/core/source/gitlab"
	"github.com/projecteru2/core/store/mock"
	"github.com/projecteru2/core/types"
	"github.com/stretchr/testify/assert"
)

func TestListPods(t *testing.T) {
	store := &mockstore.MockStore{}
	config := types.Config{}
	c := &calcium{store: store, config: config, scheduler: simplescheduler.New(), network: calico.New(), source: gitlab.New(config)}

	store.On("GetAllPods").Return([]*types.Pod{
		&types.Pod{Name: "pod1", Desc: "desc1"},
		&types.Pod{Name: "pod2", Desc: "desc2"},
	}, nil).Once()

	ps, err := c.ListPods()
	assert.Equal(t, len(ps), 2)
	assert.Nil(t, err)

	store.On("GetAllPods").Return([]*types.Pod{}, nil).Once()

	ps, err = c.ListPods()
	assert.Empty(t, ps)
	assert.Nil(t, err)
}

func TestAddPod(t *testing.T) {
	store := &mockstore.MockStore{}
	config := types.Config{}
	c := &calcium{store: store, config: config, scheduler: simplescheduler.New(), network: calico.New(), source: gitlab.New(config)}

	store.On("AddPod", "pod1", "", "desc1").Return(&types.Pod{Name: "pod1", Favor: "MEM", Desc: "desc1"}, nil)
	store.On("AddPod", "pod2", "", "desc2").Return(nil, fmt.Errorf("Etcd Error"))

	p, err := c.AddPod("pod1", "", "desc1")
	assert.Equal(t, p.Name, "pod1")
	assert.Equal(t, p.Favor, "MEM")
	assert.Equal(t, p.Desc, "desc1")
	assert.Nil(t, err)

	p, err = c.AddPod("pod2", "", "desc2")
	assert.Nil(t, p)
	assert.Equal(t, err.Error(), "Etcd Error")
}

func TestGetPods(t *testing.T) {
	store := &mockstore.MockStore{}
	config := types.Config{}
	c := &calcium{store: store, config: config, scheduler: simplescheduler.New(), network: calico.New(), source: gitlab.New(config)}

	store.On("GetPod", "pod1").Return(&types.Pod{Name: "pod1", Desc: "desc1"}, nil).Once()
	store.On("GetPod", "pod2").Return(nil, fmt.Errorf("Not found")).Once()

	p, err := c.GetPod("pod1")
	assert.Equal(t, p.Name, "pod1")
	assert.Equal(t, p.Desc, "desc1")
	assert.Nil(t, err)

	p, err = c.GetPod("pod2")
	assert.Nil(t, p)
	assert.Equal(t, err.Error(), "Not found")
}

// 后面的我实在不想写了
// 让我们相信接口都是正确的吧
