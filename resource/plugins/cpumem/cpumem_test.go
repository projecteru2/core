package cpumem

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	enginetypes "github.com/projecteru2/core/engine/types"
	plugintypes "github.com/projecteru2/core/resource/plugins/types"
	coretypes "github.com/projecteru2/core/types"
)

func TestName(t *testing.T) {
	cm := initCPUMEM(t)
	assert.Equal(t, cm.name, cm.Name())
}

func initCPUMEM(t testing.TB) *Plugin {
	config := coretypes.Config{
		Scheduler: coretypes.SchedulerConfig{
			MaxShare:  -1,
			ShareBase: 100,
		},
	}
	return NewPlugin(config, &memStore{data: map[string]string{}})
}

func generateNodes(ctx context.Context, t testing.TB, cm *Plugin, nums, cores int, memory int64, shares, index int) []string {
	reqs := generateNodeResourceRequests(t, nums, cores, memory, shares, index)
	info := &enginetypes.Info{NCPU: 8, MemTotal: 2048}
	names := []string{}
	for name, req := range reqs {
		_, err := cm.AddNode(ctx, name, req, info)
		assert.NoError(t, err)
		names = append(names, name)
	}
	t.Cleanup(func() {
		for name := range reqs {
			cm.RemoveNode(ctx, name)
		}
	})
	return names
}

func generateNodeResourceRequests(t testing.TB, nums, cores int, memory int64, shares, index int) map[string]plugintypes.NodeResourceRequest {
	infos := map[string]plugintypes.NodeResourceRequest{}
	for i := index; i < index+nums; i++ {
		info := plugintypes.NodeResourceRequest{
			"cpu":    cores,
			"share":  shares,
			"memory": fmt.Sprintf("%v", memory),
		}
		infos[fmt.Sprintf("test%v", i)] = info
	}
	return infos
}

type memStore struct {
	mu   sync.Mutex
	data map[string]string
}

func (s *memStore) NotFound(err error) bool {
	return errors.Is(err, coretypes.ErrInvaildCount)
}

func (s *memStore) GetMulti(_ context.Context, keys []string) (map[string]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data := make(map[string]string, len(keys))
	for _, key := range keys {
		value, ok := s.data[key]
		if !ok {
			return nil, coretypes.ErrInvaildCount
		}
		data[key] = value
	}
	return data, nil
}

func (s *memStore) Put(_ context.Context, data map[string]string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	maps.Copy(s.data, data)
	return nil
}

func (s *memStore) Delete(_ context.Context, keys []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, key := range keys {
		delete(s.data, key)
	}
	return nil
}
