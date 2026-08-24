package metrics

import (
	"sync"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"

	plugintypes "github.com/projecteru2/core/resource/plugins/types"
)

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

func TestSendMetricsConcurrentStatsd(t *testing.T) {
	gauge := prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "concurrent_gauge"}, []string{"nodename"})
	m := &Metrics{
		StatsdAddr: "127.0.0.1:18125",
		Collectors: map[string]prometheus.Collector{"concurrent_gauge": gauge},
	}

	var wg sync.WaitGroup
	for range 16 {
		wg.Go(func() {
			m.SendMetrics(t.Context(), &plugintypes.Metrics{
				Name: "concurrent_gauge", Labels: []string{"n1"}, Key: "k", Value: "1",
			})
		})
	}
	wg.Wait()

	assert.Equal(t, 1, collected(gauge))
}

func collected(c prometheus.Collector) int {
	ch := make(chan prometheus.Metric, 16)
	c.Collect(ch)
	close(ch)
	return len(ch)
}
