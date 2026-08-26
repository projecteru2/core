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

func TestResourceMiddlewareRefreshesNodesSequentially(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		nodes := make(chan *types.Node, 2)
		nodes <- &types.Node{NodeMeta: types.NodeMeta{Name: "n1"}}
		nodes <- &types.Node{NodeMeta: types.NodeMeta{Name: "n2"}}
		close(nodes)

		cluster := &clustermocks.Cluster{}
		cluster.On("ListPodNodes", mock.Anything, mock.Anything).Return((<-chan *types.Node)(nodes), nil).Once()
		rmgr := &resourcemocks.Manager{}
		firstStarted := make(chan struct{})
		secondStarted := make(chan struct{})
		releaseFirst := make(chan struct{})
		rmgr.On("GetNodeMetrics", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
			node := args.Get(1).(*types.Node)
			if node.Name == "n1" {
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
			t.Error("second node refresh started before the first completed")
		default:
		}

		close(releaseFirst)
		synctest.Wait()
		select {
		case <-secondStarted:
		default:
			t.Error("second node refresh did not start after the first completed")
		}
		select {
		case <-served:
		default:
			t.Error("scrape handler was not served")
		}
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
