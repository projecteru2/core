package metrics

import (
	"os"
	"strconv"
	"sync"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/resources"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"

	statsdlib "github.com/CMGS/statsd"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/exp/maps"
)

const (
	deployCountKey  = "core.%s.deploy.count"
	deployCountName = "core_deploy"
	gaugeType       = "gauge"
	counterType     = "counter"
)

// Metrics define metrics
type Metrics struct {
	Config types.Config

	StatsdAddr   string
	Hostname     string
	statsdClient *statsdlib.Client

	Collectors map[string]prometheus.Collector

	rmgr resources.Manager
}

// SendDeployCount update deploy counter
func (m *Metrics) SendDeployCount(n int) {
	log.Info("[Metrics] Update deploy counter")
	metrics := &resources.Metrics{
		Name:   deployCountName,
		Labels: []string{m.Hostname},
		Key:    deployCountKey,
		Value:  strconv.Itoa(n),
	}

	m.SendMetrics(metrics)
}

// SendMetrics update metrics
func (m *Metrics) SendMetrics(metrics ...*resources.Metrics) {
	for _, metric := range metrics {
		collector, ok := m.Collectors[metric.Name]
		if !ok {
			log.Errorf(nil, "[SendMetrics] Collector not found: %s", metric.Name) //nolint
			continue
		}
		switch collector.(type) { // nolint
		case *prometheus.GaugeVec:
			value, err := strconv.ParseFloat(metric.Value, 64)
			if err != nil {
				log.Errorf(nil, "[SendMetrics] Error occurred while parsing %v value %v: %v", metric.Name, metric.Value, err) //nolint
			}
			collector.(*prometheus.GaugeVec).WithLabelValues(metric.Labels...).Set(value) // nolint
			if err := m.gauge(metric.Key, value); err != nil {
				log.Errorf(nil, "[SendMetrics] Error occurred while sending %v data to statsd: %v", metric.Name, err) //nolint
			}
		case *prometheus.CounterVec:
			value, err := strconv.ParseInt(metric.Value, 10, 32) // nolint
			if err != nil {
				log.Errorf(nil, "[SendMetrics] Error occurred while parsing %v value %v: %v", metric.Name, metric.Value, err) //nolint
			}
			collector.(*prometheus.CounterVec).WithLabelValues(metric.Labels...).Add(float64(value)) // nolint
			if err := m.count(metric.Key, int(value), 1.0); err != nil {
				log.Errorf(nil, "[SendMetrics] Error occurred while sending %v data to statsd: %v", metric.Name, err) //nolint
			}
		default:
			log.Errorf(nil, "[SendMetrics] Unknown collector type: %T", collector) // nolint
		}
	}
}

// Lazy connect
func (m *Metrics) checkConn() error {
	if m.statsdClient != nil {
		return nil
	}
	var err error
	// We needn't try to renew/reconnect because of only supporting UDP protocol now
	// We should add an `errorCount` to reconnect when implementing TCP protocol
	if m.statsdClient, err = statsdlib.New(m.StatsdAddr, statsdlib.WithErrorHandler(func(err error) {
		log.Errorf(nil, "[statsd] Sending statsd failed: %v", err) //nolint
	})); err != nil {
		log.Errorf(nil, "[statsd] Connect statsd failed: %v", err) //nolint
		return err
	}
	return nil
}

func (m *Metrics) gauge(key string, value float64) error {
	if m.StatsdAddr == "" {
		return nil
	}
	if err := m.checkConn(); err != nil {
		return err
	}
	m.statsdClient.Gauge(key, value)
	return nil
}

func (m *Metrics) count(key string, n int, rate float32) error {
	if m.StatsdAddr == "" {
		return nil
	}
	if err := m.checkConn(); err != nil {
		return err
	}
	m.statsdClient.Count(key, n, rate)
	return nil
}

// Client is a metrics obj
var Client = Metrics{}
var once sync.Once

// InitMetrics new a metrics obj
func InitMetrics(config types.Config, metricsDescriptions []*resources.MetricsDescription) error {
	hostname, err := os.Hostname()
	if err != nil {
		return err
	}
	rmgr, err := resources.NewPluginsManager(config)
	if err != nil {
		return err
	}
	Client = Metrics{
		Config:     config,
		StatsdAddr: config.Statsd,
		Hostname:   utils.CleanStatsdMetrics(hostname),
		Collectors: map[string]prometheus.Collector{},
		rmgr:       rmgr,
	}

	for _, desc := range metricsDescriptions {
		switch desc.Type {
		case gaugeType:
			collector := prometheus.NewGaugeVec(prometheus.GaugeOpts{
				Name: desc.Name,
				Help: desc.Help,
			}, desc.Labels)
			Client.Collectors[desc.Name] = collector
		case counterType:
			collector := prometheus.NewCounterVec(prometheus.CounterOpts{
				Name: desc.Name,
				Help: desc.Help,
			}, desc.Labels)
			Client.Collectors[desc.Name] = collector
		}
	}

	Client.Collectors[deployCountName] = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: deployCountName,
		Help: "core deploy counter",
	}, []string{"hostname"})

	once.Do(func() {
		prometheus.MustRegister(maps.Values(Client.Collectors)...)
	})
	return nil
}
