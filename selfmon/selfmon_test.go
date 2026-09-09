package selfmon

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	clustermocks "github.com/projecteru2/core/cluster/mocks"
	storemocks "github.com/projecteru2/core/store/mocks"
	"github.com/projecteru2/core/types"
	walmocks "github.com/projecteru2/core/wal/mocks"
)

const (
	testConnectionTimeout = 10 * time.Second
	testKeepaliveInterval = 16 * time.Second
	testRetryInterval     = testKeepaliveInterval / 4
	testHeartbeatInterval = 15 * time.Second
	testOverloadedNodes   = nodeStatusHandlers + 4
)

func TestWithActiveLockCancelsTheWorkWhenTheLockExpires(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		expiry := make(chan struct{})
		var unregistered atomic.Bool
		store := &storemocks.Store{}
		store.On("StartEphemeral", mock.Anything, ActiveKey, testKeepaliveInterval).
			Return((<-chan struct{})(expiry), func() { unregistered.Store(true) }, nil).
			Once()

		n := &NodeStatusWatcher{config: testConfig(), store: store}
		entered := make(chan struct{})
		canceled := make(chan struct{})
		done := make(chan struct{})
		go func() {
			defer close(done)
			n.withActiveLock(t.Context(), func(ctx context.Context) {
				close(entered)
				<-ctx.Done()
				close(canceled)
			})
		}()

		synctest.Wait()
		assert.True(t, isClosed(entered), "withActiveLock did not run the work under the lock")
		assert.False(t, isClosed(canceled), "the work context was canceled while the lock was held")

		close(expiry)
		synctest.Wait()
		assert.True(t, isClosed(canceled), "the work context outlived the lock")
		assert.True(t, isClosed(done), "withActiveLock did not return after the lock expired")
		assert.True(t, unregistered.Load(), "the expired lock was not unregistered")
		store.AssertExpectations(t)
	})
}

func TestWithActiveLockRetriesAfterALostElection(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		expiry := make(chan struct{})
		store := &storemocks.Store{}
		store.On("StartEphemeral", mock.Anything, ActiveKey, testKeepaliveInterval).Return(nil, nil, types.ErrKeyExists).Once()
		store.On("StartEphemeral", mock.Anything, ActiveKey, testKeepaliveInterval).Return(nil, nil, types.ErrMockError).Once()
		store.On("StartEphemeral", mock.Anything, ActiveKey, testKeepaliveInterval).
			Return((<-chan struct{})(expiry), func() {}, nil).
			Once()

		n := &NodeStatusWatcher{config: testConfig(), store: store}
		active := make(chan struct{})
		done := make(chan struct{})
		go func() {
			defer close(done)
			n.withActiveLock(t.Context(), func(context.Context) { close(active) })
		}()

		synctest.Wait()
		store.AssertNumberOfCalls(t, "StartEphemeral", 1)

		time.Sleep(testRetryInterval - time.Millisecond)
		synctest.Wait()
		store.AssertNumberOfCalls(t, "StartEphemeral", 1)

		time.Sleep(time.Millisecond)
		synctest.Wait()
		store.AssertNumberOfCalls(t, "StartEphemeral", 2)
		assert.False(t, isClosed(active), "the work ran without an active lock")

		time.Sleep(testRetryInterval)
		synctest.Wait()
		store.AssertNumberOfCalls(t, "StartEphemeral", 3)
		assert.True(t, isClosed(active), "the work did not run after winning the election")
		assert.True(t, isClosed(done), "withActiveLock did not return")
		store.AssertExpectations(t)
	})
}

func TestRunStopsWhenTheContextIsCanceled(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		messages := make(chan *types.NodeStatus)
		close(messages)
		expiry := make(chan struct{})
		store := &storemocks.Store{}
		store.On("StartEphemeral", mock.Anything, ActiveKey, testKeepaliveInterval).Return((<-chan struct{})(expiry), func() {}, nil)
		store.On("GetNodesByPod", mock.Anything, mock.Anything, true).Return(nil, nil).Maybe()
		store.On("GetServiceStatus", mock.Anything).Return(nil, nil).Maybe()
		cluster := &clustermocks.Cluster{}
		cluster.On("NodeStatusStream", mock.Anything).Return(messages)

		n := &NodeStatusWatcher{config: testConfig(), store: store, cluster: cluster, wal: &walmocks.WAL{}}
		ctx, cancel := context.WithCancel(t.Context())
		done := make(chan struct{})
		go func() {
			defer close(done)
			n.run(ctx)
		}()

		synctest.Wait()
		store.AssertNumberOfCalls(t, "StartEphemeral", 1)
		assert.False(t, isClosed(done), "run returned before the context was canceled")

		time.Sleep(testConnectionTimeout)
		synctest.Wait()
		store.AssertNumberOfCalls(t, "StartEphemeral", 2)
		assert.False(t, isClosed(done), "run returned before the context was canceled")

		cancel()
		synctest.Wait()
		assert.True(t, isClosed(done), "run did not return after the context was canceled")
	})
}

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

func TestReplayDeadJournalsSkipsTakeoverOnStatusError(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		store := &storemocks.Store{}
		store.On("GetServiceStatus", mock.Anything).Return(nil, types.ErrMockError)
		mwal := &walmocks.WAL{}
		mwal.On("Takeover", mock.Anything, mock.Anything).Return().Maybe()

		n := &NodeStatusWatcher{config: testConfig(), store: store, wal: mwal}
		ctx, cancel := context.WithCancel(t.Context())
		done := make(chan struct{})
		go func() {
			defer close(done)
			n.replayDeadJournals(ctx)
		}()

		time.Sleep(3*testHeartbeatInterval + time.Millisecond)
		synctest.Wait()
		store.AssertNumberOfCalls(t, "GetServiceStatus", 3)
		mwal.AssertNotCalled(t, "Takeover", mock.Anything, mock.Anything)

		cancel()
		synctest.Wait()
		assert.True(t, isClosed(done), "replayDeadJournals did not return after the context was canceled")
	})
}

func TestInitNodeStatusDealsOnlyWithUnavailableNodes(t *testing.T) {
	nodes := []*types.Node{
		downNode("down-1"),
		{NodeMeta: types.NodeMeta{Name: "available", Podname: "pod"}, Available: true},
		{NodeMeta: types.NodeMeta{Name: "test", Podname: "pod"}, Test: true},
		downNode("down-2"),
	}
	store := &storemocks.Store{}
	store.On("GetNodesByPod", mock.Anything, mock.MatchedBy(func(filter *types.NodeFilter) bool { return filter.All }), true).
		Return(nodes, nil).
		Once()
	cluster := &clustermocks.Cluster{}
	cluster.On("SetNode", mock.Anything, &types.SetNodeOptions{Nodename: "down-1", WorkloadsDown: true}).Return(nil, nil).Once()
	cluster.On("SetNode", mock.Anything, &types.SetNodeOptions{Nodename: "down-2", WorkloadsDown: true}).Return(nil, nil).Once()
	cluster.On("SetNode", mock.Anything, mock.Anything).Return(nil, nil).Maybe()

	n := &NodeStatusWatcher{config: testConfig(), store: store, cluster: cluster}
	n.initNodeStatus(t.Context())

	cluster.AssertNumberOfCalls(t, "SetNode", 2)
	store.AssertExpectations(t)
	cluster.AssertExpectations(t)
}

func TestInitNodeStatusBoundsConcurrentHandlers(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		nodes := make([]*types.Node, testOverloadedNodes)
		for i := range nodes {
			nodes[i] = downNode(fmt.Sprintf("down-%d", i))
		}
		store := &storemocks.Store{}
		store.On("GetNodesByPod", mock.Anything, mock.Anything, true).Return(nodes, nil).Once()
		release := make(chan struct{})
		inflight := make(chan struct{}, testOverloadedNodes)
		cluster := &clustermocks.Cluster{}
		cluster.On("SetNode", mock.Anything, mock.Anything).Run(func(mock.Arguments) {
			inflight <- struct{}{}
			<-release
		}).Return(nil, nil)

		n := &NodeStatusWatcher{config: testConfig(), store: store, cluster: cluster}
		done := make(chan struct{})
		go func() {
			defer close(done)
			n.initNodeStatus(t.Context())
		}()

		synctest.Wait()
		assert.Len(t, inflight, nodeStatusHandlers)

		close(release)
		synctest.Wait()
		assert.Len(t, inflight, testOverloadedNodes)
		assert.True(t, isClosed(done), "initNodeStatus did not wait for its handlers")
		store.AssertExpectations(t)
	})
}

func TestInitNodeStatusStopsOnStoreError(t *testing.T) {
	store := &storemocks.Store{}
	store.On("GetNodesByPod", mock.Anything, mock.Anything, true).Return(nil, types.ErrMockError).Once()
	cluster := &clustermocks.Cluster{}

	n := &NodeStatusWatcher{config: testConfig(), store: store, cluster: cluster}
	n.initNodeStatus(t.Context())

	cluster.AssertNotCalled(t, "SetNode", mock.Anything, mock.Anything)
	store.AssertExpectations(t)
}

func TestMonitorReturnsWhenTheStreamCloses(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		messages := make(chan *types.NodeStatus, 1)
		messages <- &types.NodeStatus{Nodename: "down-1", Podname: "pod"}
		close(messages)
		store := &storemocks.Store{}
		store.On("GetNodesByPod", mock.Anything, mock.Anything, true).Return(nil, nil).Maybe()
		cluster := &clustermocks.Cluster{}
		cluster.On("NodeStatusStream", mock.Anything).Return(messages).Once()
		cluster.On("SetNode", mock.Anything, &types.SetNodeOptions{Nodename: "down-1", WorkloadsDown: true}).Return(nil, nil).Once()

		n := &NodeStatusWatcher{config: testConfig(), store: store, cluster: cluster}
		err := n.monitor(t.Context())
		synctest.Wait()

		assert.ErrorIs(t, err, types.ErrMessageChanClosed)
		cluster.AssertExpectations(t)
	})
}

func TestMonitorReturnsWhenTheContextEnds(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		messages := make(chan *types.NodeStatus)
		store := &storemocks.Store{}
		store.On("GetNodesByPod", mock.Anything, mock.Anything, true).Return(nil, nil).Maybe()
		cluster := &clustermocks.Cluster{}
		cluster.On("NodeStatusStream", mock.Anything).Return(messages).Once()

		n := &NodeStatusWatcher{config: testConfig(), store: store, cluster: cluster}
		ctx, cancel := context.WithCancel(t.Context())
		errs := make(chan error, 1)
		go func() { errs <- n.monitor(ctx) }()

		synctest.Wait()
		assert.Empty(t, errs, "monitor returned while the stream was open")

		cancel()
		synctest.Wait()
		require.Len(t, errs, 1)
		assert.ErrorIs(t, <-errs, context.Canceled)
		cluster.AssertExpectations(t)
	})
}

func TestDealNodeStatusMessageSetsWorkloadsDownOnlyForDeadNodes(t *testing.T) {
	for _, tt := range []struct {
		name        string
		message     *types.NodeStatus
		setNodeErr  error
		wantSetNode bool
	}{
		{"a dead node has its workloads set down", &types.NodeStatus{Nodename: "dead", Podname: "pod"}, nil, true},
		{"a failed set node is swallowed", &types.NodeStatus{Nodename: "dead", Podname: "pod"}, types.ErrMockError, true},
		{"an alive node is left to the agent", &types.NodeStatus{Nodename: "alive", Podname: "pod", Alive: true}, nil, false},
		{"a broken message is dropped", &types.NodeStatus{Nodename: "broken", Podname: "pod", Error: types.ErrMockError}, nil, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cluster := &clustermocks.Cluster{}
			cluster.On("SetNode", mock.Anything, mock.Anything).Return(nil, tt.setNodeErr).Maybe()

			n := &NodeStatusWatcher{config: testConfig(), cluster: cluster}
			n.dealNodeStatusMessage(t.Context(), tt.message)

			if !tt.wantSetNode {
				cluster.AssertNotCalled(t, "SetNode", mock.Anything, mock.Anything)
				return
			}
			cluster.AssertNumberOfCalls(t, "SetNode", 1)
			cluster.AssertCalled(t, "SetNode", mock.Anything, &types.SetNodeOptions{Nodename: tt.message.Nodename, WorkloadsDown: true})
		})
	}
}

func TestRunNodeStatusWatcherStopsWhenTheContextEnds(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		store := &storemocks.Store{}
		store.On("StartEphemeral", mock.Anything, ActiveKey, testKeepaliveInterval).Return(nil, nil, types.ErrKeyExists)

		ctx, cancel := context.WithCancel(t.Context())
		done := make(chan struct{})
		go func() {
			defer close(done)
			RunNodeStatusWatcher(ctx, testConfig(), &clustermocks.Cluster{}, store, &walmocks.WAL{})
		}()

		synctest.Wait()
		store.AssertNumberOfCalls(t, "StartEphemeral", 1)
		assert.False(t, isClosed(done), "RunNodeStatusWatcher returned before the context ended")

		cancel()
		synctest.Wait()
		assert.True(t, isClosed(done), "RunNodeStatusWatcher did not return when the context ended")
		store.AssertExpectations(t)
	})
}

func testConfig() types.Config {
	return types.Config{
		GlobalTimeout:       time.Minute,
		ConnectionTimeout:   testConnectionTimeout,
		HAKeepaliveInterval: testKeepaliveInterval,
		GRPCConfig:          types.GRPCConfig{ServiceHeartbeatInterval: testHeartbeatInterval},
	}
}

func downNode(name string) *types.Node {
	return &types.Node{NodeMeta: types.NodeMeta{Name: name, Podname: "pod"}}
}

func isClosed(ch <-chan struct{}) bool {
	select {
	case <-ch:
		return true
	default:
		return false
	}
}
