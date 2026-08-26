package selfmon

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"

	storemocks "github.com/projecteru2/core/store/mocks"
	"github.com/projecteru2/core/types"
	walmocks "github.com/projecteru2/core/wal/mocks"
)

func TestReplayDeadJournalsUsesFreshServiceList(t *testing.T) {
	store := &storemocks.Store{}
	mwal := &walmocks.WAL{}
	n := &NodeStatusWatcher{
		config: types.Config{GRPCConfig: types.GRPCConfig{ServiceHeartbeatInterval: 10 * time.Millisecond}},
		store:  store,
		wal:    mwal,
	}

	store.On("GetServiceStatus", mock.Anything).Return(nil, types.ErrMockError).Once()
	store.On("GetServiceStatus", mock.Anything).Return([]string{"127.0.0.1:5001"}, nil)
	var takeovers atomic.Int32
	done := make(chan struct{})
	mwal.On("Takeover", mock.Anything, []string{"127.0.0.1:5001"}).Run(func(mock.Arguments) {
		if takeovers.Add(1) == 2 {
			close(done)
		}
	}).Return()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go n.replayDeadJournals(ctx)

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("takeover did not run with the fresh service list")
	}
	store.AssertExpectations(t)
}
