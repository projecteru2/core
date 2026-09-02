package metrics

import (
	"net/http"
	"net/http/httptest"
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

func TestResourceMiddlewareRefreshesNodesConcurrently(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		cluster := &clustermocks.Cluster{}
		cluster.On("ListPodNodes", mock.Anything, mock.Anything).Return(twoNodes(), nil).Once()
		rmgr := &resourcemocks.Manager{}
		firstStarted := make(chan struct{})
		secondStarted := make(chan struct{})
		releaseFirst := make(chan struct{})
		rmgr.On("GetNodeMetrics", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
			if args.Get(1).(*types.Node).Name == "n1" {
				close(firstStarted)
				<-releaseFirst
				return
			}
			close(secondStarted)
		}).Return(nil, nil).Twice()

		m := &Metrics{Config: types.Config{GlobalTimeout: time.Second}, rmgr: rmgr}
		served := make(chan struct{})
		handler := m.ResourceMiddleware(cluster)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			close(served)
		}))
		go handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/metrics", nil))

		<-firstStarted
		synctest.Wait()
		select {
		case <-secondStarted:
		default:
			t.Error("second node refresh waited for the first")
		}
		select {
		case <-served:
			t.Error("scrape handler served before every node was refreshed")
		default:
		}

		close(releaseFirst)
		synctest.Wait()
		select {
		case <-served:
		default:
			t.Error("scrape handler was not served")
		}
	})
}

func TestResourceMiddlewareSharesOneRefreshBetweenOverlappingScrapes(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		cluster := &clustermocks.Cluster{}
		cluster.On("ListPodNodes", mock.Anything, mock.Anything).Return(twoNodes(), nil).Once()
		rmgr := &resourcemocks.Manager{}
		release := make(chan struct{})
		rmgr.On("GetNodeMetrics", mock.Anything, mock.Anything).Run(func(mock.Arguments) { <-release }).Return(nil, nil).Twice()

		m := &Metrics{Config: types.Config{GlobalTimeout: time.Second}, rmgr: rmgr}
		served := make(chan struct{}, 2)
		handler := m.ResourceMiddleware(cluster)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
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

func TestResourceMiddlewareListNodesFailed(t *testing.T) {
	cluster := &clustermocks.Cluster{}
	cluster.On("ListPodNodes", mock.Anything, mock.Anything).Return(nil, errors.New("etcd unavailable"))

	m := &Metrics{Config: types.Config{GlobalTimeout: time.Second}}
	served := make(chan struct{})
	handler := m.ResourceMiddleware(cluster)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
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
