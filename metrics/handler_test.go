package metrics

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/prometheus/client_golang/prometheus"

	clustermocks "github.com/projecteru2/core/cluster/mocks"
	plugintypes "github.com/projecteru2/core/resource/plugins/types"
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

func TestSendMetricsSkipsUnparsableValues(t *testing.T) {
	gauge := prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "test_gauge"}, []string{"nodename"})
	counter := prometheus.NewCounterVec(prometheus.CounterOpts{Name: "test_counter"}, []string{"nodename"})
	m := &Metrics{Collectors: map[string]prometheus.Collector{"test_gauge": gauge, "test_counter": counter}}

	m.SendMetrics(t.Context(),
		&plugintypes.Metrics{Name: "test_gauge", Labels: []string{"n1"}, Key: "k", Value: "not-a-number"},
		&plugintypes.Metrics{Name: "test_counter", Labels: []string{"n1"}, Key: "k", Value: "not-a-number"},
	)

	assert.Equal(t, 0, collected(gauge))
	assert.Equal(t, 0, collected(counter))
}

func collected(c prometheus.Collector) int {
	ch := make(chan prometheus.Metric, 16)
	c.Collect(ch)
	close(ch)
	return len(ch)
}
