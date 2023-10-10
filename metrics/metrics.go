package metrics

import (
	"context"
	"os"
	"strconv"
	"sync"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/resource"
	"github.com/projecteru2/core/resource/cobalt"
	plugintypes "github.com/projecteru2/core/resource/plugins/types"
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

	rmgr resource.Manager
}

// SendDeployCount update deploy counter
func (m *Metrics) SendDeployCount(ctx context.Context, n int) {
	log.WithFunc("metrics.SendDeployCount").Info(ctx, "Update deploy counter")
	metrics := &plugintypes.Metrics{
		Name:   deployCountName,
		Labels: []string{m.Hostname},
		Key:    deployCountKey,
		Value:  strconv.Itoa(n),
	}

	m.SendMetrics(ctx, metrics)
}

// SendMetrics update metrics
func (m *Metrics) SendMetrics(ctx context.Context, metrics ...*plugintypes.Metrics) {
	logger := log.WithFunc("metrics.SendMetrics")
	for _, metric := range metrics {
		collector, ok := m.Collectors[metric.Name]
		if !ok {
			logger.Warnf(ctx, "Collector not found: %s", metric.Name)
			continue
		}
		switch collector.(type) { //nolint
		case *prometheus.GaugeVec:
			value, err := strconv.ParseFloat(metric.Value, 64)
			if err != nil {
				logger.Errorf(ctx, err, "Error occurred while parsing %+v value %+v", metric.Name, metric.Value)
			}
			collector.(*prometheus.GaugeVec).WithLabelValues(metric.Labels...).Set(value) //nolint
			if err := m.gauge(ctx, metric.Key, value); err != nil {
				logger.Errorf(ctx, err, "Error occurred while sending %+v data to statsd", metric.Name)
			}
		case *prometheus.CounterVec:
			value, err := strconv.ParseInt(metric.Value, 10, 32) //nolint
			if err != nil {
				logger.Errorf(ctx, err, "Error occurred while parsing %+v value %+v", metric.Name, metric.Value)
			}
			collector.(*prometheus.CounterVec).WithLabelValues(metric.Labels...).Add(float64(value)) //nolint
			if err := m.count(ctx, metric.Key, int(value), 1.0); err != nil {
				logger.Errorf(ctx, err, "Error occurred while sending %+v data to statsd", metric.Name)
			}
		default:
			logger.Errorf(ctx, types.ErrMetricsTypeNotSupport, "Unknown collector type: %T", collector)
		}
	}
}

// ResetCollectors 清除旧的 Collectors 并创建新的 Collectors
func (m *Metrics) ResetCollectors(metricsDescriptions []*plugintypes.MetricsDescription) {
	if len(metricsDescriptions) == 0 {
		return
	}
	if len(m.Collectors) > 0 {
		for _, oldCollector := range m.Collectors {
			prometheus.Unregister(oldCollector)
		}
	}

	// 创建新的 Collectors map
	newCollectors := map[string]prometheus.Collector{}
	// 重新创建新的 Collectors
	for _, desc := range metricsDescriptions {
		switch desc.Type {
		case gaugeType:
			collector := prometheus.NewGaugeVec(prometheus.GaugeOpts{
				Name: desc.Name,
				Help: desc.Help,
			}, desc.Labels)
			newCollectors[desc.Name] = collector
		case counterType:
			collector := prometheus.NewCounterVec(prometheus.CounterOpts{
				Name: desc.Name,
				Help: desc.Help,
			}, desc.Labels)
			newCollectors[desc.Name] = collector
		}
	}

	// 将新的 Collectors 添加到 Metrics 结构中
	m.Collectors = newCollectors
	// 重新注册新的 Collectors
	prometheus.MustRegister(maps.Values(m.Collectors)...)
}

// Lazy connect
func (m *Metrics) checkConn(ctx context.Context) error {
	if m.statsdClient != nil {
		return nil
	}
	logger := log.WithFunc("metrics.checkConn")
	var err error
	// We needn't try to renew/reconnect because of only supporting UDP protocol now
	// We should add an `errorCount` to reconnect when implementing TCP protocol
	if m.statsdClient, err = statsdlib.New(m.StatsdAddr, statsdlib.WithErrorHandler(func(err error) {
		logger.Error(ctx, err, "Sending statsd failed")
	})); err != nil {
		logger.Error(ctx, err, "Connect statsd failed")
		return err
	}
	return nil
}

func (m *Metrics) gauge(ctx context.Context, key string, value float64) error {
	if m.StatsdAddr == "" {
		return nil
	}
	if err := m.checkConn(ctx); err != nil {
		return err
	}
	m.statsdClient.Gauge(key, value)
	return nil
}

func (m *Metrics) count(ctx context.Context, key string, n int, rate float32) error {
	if m.StatsdAddr == "" {
		return nil
	}
	if err := m.checkConn(ctx); err != nil {
		return err
	}
	m.statsdClient.Count(key, n, rate)
	return nil
}

// Client is a metrics obj
var Client = Metrics{}
var once sync.Once

// InitMetrics new a metrics obj
func InitMetrics(ctx context.Context, config types.Config, metricsDescriptions []*plugintypes.MetricsDescription) error {
	hostname, err := os.Hostname()
	if err != nil {
		return err
	}
	rmgr, err := cobalt.New(config)
	if err != nil {
		return err
	}
	if err := rmgr.LoadPlugins(ctx, nil); err != nil {
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
