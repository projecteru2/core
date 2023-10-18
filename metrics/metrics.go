package metrics

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"sync"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/resource"
	"github.com/projecteru2/core/resource/cobalt"
	plugintypes "github.com/projecteru2/core/resource/plugins/types"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
	promClient "github.com/prometheus/client_model/go"
	"golang.org/x/exp/slices"

	statsdlib "github.com/CMGS/statsd"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/exp/maps"
)

const (
	deployCountKey    = "core.%s.deploy.count"
	deployCountName   = "core_deploy"
	podNodeStatusKey  = "pod.node.%s.up"
	podNodeStatusName = "pod_node_up"

	gaugeType   = "gauge"
	counterType = "counter"
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

// RemoveInvalidNodes 清除多余的metric标签值
func (m *Metrics) RemoveInvalidNodes(invalidNodes ...string) {
	if len(invalidNodes) == 0 {
		return
	}
	for _, collector := range m.Collectors {
		metrics, _ := prometheus.DefaultGatherer.Gather()
		for _, metric := range metrics {
			for _, mf := range metric.GetMetric() {
				if !slices.ContainsFunc(mf.Label, func(label *promClient.LabelPair) bool {
					return label.GetName() == "nodename" && slices.ContainsFunc(invalidNodes, func(nodename string) bool {
						return label.GetValue() == nodename
					})
				}) {
					continue
				}
				labels := prometheus.Labels{}
				for _, label := range mf.Label {
					labels[label.GetName()] = label.GetValue()
				}
				// 删除符合条件的度量标签
				switch c := collector.(type) {
				case *prometheus.GaugeVec:
					c.Delete(labels)
				case *prometheus.CounterVec:
					c.Delete(labels)
				}
			}
		}
		// 添加更多的条件来处理其他类型的Collector
	}
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

	Client.Collectors[podNodeStatusName] = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: podNodeStatusName,
		Help: "number of up nodes",
	}, []string{"hostname", "podname", "nodename"})

	once.Do(func() {
		prometheus.MustRegister(maps.Values(Client.Collectors)...)
	})
	return nil
}
