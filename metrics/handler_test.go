package metrics

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	clustermocks "github.com/projecteru2/core/cluster/mocks"
	"github.com/projecteru2/core/types"
)

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
