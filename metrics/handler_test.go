package metrics

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	clustermocks "github.com/projecteru2/core/cluster/mocks"
	resourcemocks "github.com/projecteru2/core/resource/mocks"
	"github.com/projecteru2/core/types"
)

func TestResourceMiddlewareRefreshesEveryNodeInOneCall(t *testing.T) {
	cluster := &clustermocks.Cluster{}
	cluster.On("ListPodNodes", mock.Anything, mock.Anything).Return(twoNodes(), nil).Once()
	rmgr := &resourcemocks.Manager{}
	rmgr.On("GetNodesMetrics", mock.Anything, mock.MatchedBy(func(nodes []*types.Node) bool { return len(nodes) == 2 })).Return(nil, nil).Once()

	m := &Metrics{Config: types.Config{GlobalTimeout: time.Second}, rmgr: rmgr}
	served := false
	handler := m.ResourceMiddleware(t.Context(), cluster)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		served = true
	}))
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/metrics", nil))

	assert.True(t, served)
	rmgr.AssertExpectations(t)
}

func TestResourceMiddlewareSharesOneRefreshBetweenOverlappingScrapes(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		cluster := &clustermocks.Cluster{}
		cluster.On("ListPodNodes", mock.Anything, mock.Anything).Return(twoNodes(), nil).Once()
		rmgr := &resourcemocks.Manager{}
		release := make(chan struct{})
		rmgr.On("GetNodesMetrics", mock.Anything, mock.Anything).Run(func(mock.Arguments) { <-release }).Return(nil, nil).Once()

		m := &Metrics{Config: types.Config{GlobalTimeout: time.Second}, rmgr: rmgr}
		served := make(chan struct{}, 2)
		handler := m.ResourceMiddleware(t.Context(), cluster)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			served <- struct{}{}
		}))
		for range 2 {
			go handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/metrics", nil))
		}
		synctest.Wait()
		close(release)
		synctest.Wait()

		assert.Len(t, served, 2)
		cluster.AssertExpectations(t)
		rmgr.AssertExpectations(t)
	})
}

func TestResourceMiddlewareRefreshOutlivesTheScrapeThatStartedIt(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		cluster := &clustermocks.Cluster{}
		cluster.On("ListPodNodes", mock.Anything, mock.Anything).Return(twoNodes(), nil).Once()
		rmgr := &resourcemocks.Manager{}
		release := make(chan struct{})
		var cancelled atomic.Int32
		rmgr.On("GetNodesMetrics", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
			<-release
			if args.Get(0).(context.Context).Err() != nil {
				cancelled.Add(1)
			}
		}).Return(nil, nil).Once()

		m := &Metrics{Config: types.Config{GlobalTimeout: time.Second}, rmgr: rmgr}
		served := make(chan struct{}, 2)
		handler := m.ResourceMiddleware(t.Context(), cluster)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			served <- struct{}{}
		}))
		leaderCtx, leaveLeader := context.WithCancel(t.Context())
		go handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/metrics", nil).WithContext(leaderCtx))
		synctest.Wait()
		go handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/metrics", nil))
		synctest.Wait()

		leaveLeader()
		synctest.Wait()
		assert.Empty(t, served)

		close(release)
		synctest.Wait()
		assert.Len(t, served, 1)
		assert.Zero(t, cancelled.Load())
		rmgr.AssertExpectations(t)
	})
}

func TestResourceMiddlewareListNodesFailed(t *testing.T) {
	cluster := &clustermocks.Cluster{}
	cluster.On("ListPodNodes", mock.Anything, mock.Anything).Return(nil, errors.New("etcd unavailable"))

	m := &Metrics{Config: types.Config{GlobalTimeout: time.Second}}
	served := make(chan struct{})
	handler := m.ResourceMiddleware(t.Context(), cluster)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		close(served)
	}))

	go handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/metrics", nil))

	select {
	case <-served:
	case <-time.After(5 * time.Second):
		assert.Fail(t, "scrape handler blocked after ListPodNodes failed")
	}
}

func twoNodes() <-chan *types.Node {
	nodes := make(chan *types.Node, 2)
	nodes <- &types.Node{NodeMeta: types.NodeMeta{Name: "n1"}}
	nodes <- &types.Node{NodeMeta: types.NodeMeta{Name: "n2"}}
	close(nodes)
	return nodes
}
