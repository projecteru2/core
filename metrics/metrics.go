package metrics

import (
	"context"
	"fmt"
	"maps"
	"os"
	"slices"
	"strconv"
	"sync"

	promClient "github.com/prometheus/client_model/go"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/resource"
	plugintypes "github.com/projecteru2/core/resource/plugins/types"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"

	statsdlib "github.com/CMGS/statsd"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	deployCountKey    = "core.%s.deploy.count"
	deployCountName   = "core_deploy"
	podNodeStatusKey  = "pod.node.%s.up"
	podNodeStatusName = "pod_node_up"

	gaugeType   = "gauge"
	counterType = "counter"
)

var (
	Client = Metrics{}
	once   sync.Once
)

// Metrics ships core metrics to Prometheus and statsd.
type Metrics struct {
	Config types.Config

	StatsdAddr   string
	Hostname     string
	statsdMu     sync.Mutex
	statsdClient *statsdlib.Client

	Collectors map[string]prometheus.Collector

	rmgr resource.Manager
}

func (m *Metrics) SendDeployCount(ctx context.Context, n int) {
	metrics := &plugintypes.Metrics{
		Name:   deployCountName,
		Labels: []string{m.Hostname},
		Key:    deployCountKey,
		Value:  strconv.Itoa(n),
	}

	m.SendMetrics(ctx, metrics)
}

func (m *Metrics) SendPodNodeStatus(ctx context.Context, node *types.Node) {
	up := !node.IsDown()
	metrics := &plugintypes.Metrics{
		Name:   podNodeStatusName,
		Labels: []string{m.Hostname, node.Podname, node.Name},
		Key:    fmt.Sprintf(podNodeStatusKey, node.Name),
		Value:  strconv.Itoa(utils.Bool2Int(up)),
	}

	m.SendMetrics(ctx, metrics)
}

func (m *Metrics) SendMetrics(ctx context.Context, metrics ...*plugintypes.Metrics) {
	logger := log.WithFunc("metrics.SendMetrics")
	for _, metric := range metrics {
		collector, ok := m.Collectors[metric.Name]
		if !ok {
			logger.Warnf(ctx, "collector not found: %s", metric.Name)
			continue
		}
		switch c := collector.(type) {
		case *prometheus.GaugeVec:
			value, err := strconv.ParseFloat(metric.Value, 64)
			if err != nil {
				logger.Errorf(ctx, err, "failed to parse %s value %s", metric.Name, metric.Value)
				continue
			}
			c.WithLabelValues(metric.Labels...).Set(value)
			if err := m.gauge(ctx, metric.Key, value); err != nil {
				logger.Errorf(ctx, err, "failed to send %s to statsd", metric.Name)
			}
		case *prometheus.CounterVec:
			value, err := strconv.ParseInt(metric.Value, 10, 32)
			if err != nil {
				logger.Errorf(ctx, err, "failed to parse %s value %s", metric.Name, metric.Value)
				continue
			}
			c.WithLabelValues(metric.Labels...).Add(float64(value))
			if err := m.count(ctx, metric.Key, int(value), 1.0); err != nil {
				logger.Errorf(ctx, err, "failed to send %s to statsd", metric.Name)
			}
		default:
			logger.Errorf(ctx, types.ErrMetricsTypeNotSupport, "unknown collector type: %T", collector)
		}
	}
}

// RemoveInvalidNodes drops Prometheus label sets for a node that no longer exists.
func (m *Metrics) RemoveInvalidNodes(invalidNode string) {
	metrics, _ := prometheus.DefaultGatherer.Gather()
	for _, collector := range m.Collectors {
		for _, metric := range metrics {
			for _, mf := range metric.GetMetric() {
				if !slices.ContainsFunc(mf.Label, func(label *promClient.LabelPair) bool {
					return label.GetName() == "nodename" && label.GetValue() == invalidNode
				}) {
					continue
				}
				labels := prometheus.Labels{}
				for _, label := range mf.Label {
					labels[label.GetName()] = label.GetValue()
				}
				switch c := collector.(type) {
				case *prometheus.GaugeVec:
					c.Delete(labels)
				case *prometheus.CounterVec:
					c.Delete(labels)
				}
			}
		}
	}
}

func (m *Metrics) client(ctx context.Context) (*statsdlib.Client, error) {
	m.statsdMu.Lock()
	defer m.statsdMu.Unlock()
	if m.statsdClient != nil {
		return m.statsdClient, nil
	}
	logger := log.WithFunc("metrics.client")
	var err error
	// UDP is connectionless, so a failed client never needs reconnecting
	if m.statsdClient, err = statsdlib.New(m.StatsdAddr, statsdlib.WithErrorHandler(func(err error) {
		logger.Error(ctx, err, "failed to send to statsd")
	})); err != nil {
		logger.Error(ctx, err, "failed to connect statsd")
		return nil, err
	}
	return m.statsdClient, nil
}

func (m *Metrics) gauge(ctx context.Context, key string, value float64) error {
	if m.StatsdAddr == "" {
		return nil
	}
	c, err := m.client(ctx)
	if err != nil {
		return err
	}
	c.Gauge(key, value)
	return nil
}

func (m *Metrics) count(ctx context.Context, key string, n int, rate float32) error {
	if m.StatsdAddr == "" {
		return nil
	}
	c, err := m.client(ctx)
	if err != nil {
		return err
	}
	c.Count(key, n, rate)
	return nil
}

// InitMetrics builds the global metrics client and registers its collectors.
func InitMetrics(config types.Config, rmgr resource.Manager, metricsDescriptions []*plugintypes.MetricsDescription) error {
	hostname, err := os.Hostname()
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

	Client.Collectors[podNodeStatusName] = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: podNodeStatusName,
		Help: "node status",
	}, []string{"hostname", "podname", "nodename"})

	once.Do(func() {
		prometheus.MustRegister(slices.Collect(maps.Values(Client.Collectors))...)
	})
	return nil
}
