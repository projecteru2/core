package metrics

import (
	"fmt"
	"os"

	statsdlib "github.com/CMGS/statsd"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

const (
	cpuMap      = "core.node.%s.cpu.%s"
	memStats    = "core.node.%s.memory"
	deployCount = "core.%s.deploy.count"
)

// Metrics define metrics
type Metrics struct {
	StatsdAddr   string
	Hostname     string
	statsdClient *statsdlib.Client

	MemoryCapacity *prometheus.GaugeVec
	CPUMap         *prometheus.GaugeVec
	DeployCount    *prometheus.CounterVec
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
		log.Errorf("[statsd] Sending statsd failed: %v", err)
	})); err != nil {
		log.Errorf("[statsd] Connect statsd failed: %v", err)
		return err
	}
	return nil
}

func (m *Metrics) gauge(key string, value float64) error {
	if err := m.checkConn(); err != nil {
		return err
	}
	m.statsdClient.Gauge(key, value)
	return nil
}

func (m *Metrics) count(key string, n int, rate float32) error {
	if err := m.checkConn(); err != nil {
		return err
	}
	m.statsdClient.Count(key, n, rate)
	return nil
}

// SendNodeInfo update node resource capacity
func (m *Metrics) SendNodeInfo(node *types.Node) {
	log.Debugf("[Metrics] Update %s memory capacity gauge", node.Name)
	nodename := utils.CleanStatsdMetrics(node.Name)
	memory := float64(node.MemCap)

	if m.MemoryCapacity != nil {
		m.MemoryCapacity.WithLabelValues(nodename).Set(memory)
	}

	for cpuid, value := range node.CPU {
		val := float64(value)
		key := fmt.Sprintf(cpuMap, nodename, cpuid)
		if m.CPUMap != nil {
			m.CPUMap.WithLabelValues(nodename, cpuid).Set(val)
		}
		if m.StatsdAddr == "" {
			continue
		}
		if err := m.gauge(key, val); err != nil {
			log.Errorf("[SendNodeInfo] Error occurred while sending data to statsd: %v", err)
		}
	}

	if m.StatsdAddr == "" {
		return
	}
	key := fmt.Sprintf(memStats, nodename)
	if err := m.gauge(key, memory); err != nil {
		log.Errorf("[SendNodeInfo] Error occurred while sending data to statsd: %v", err)
	}
}

// SendDeployCount update deploy counter
func (m *Metrics) SendDeployCount(n int) {
	log.Info("[Metrics] Update deploy counter")
	if m.DeployCount != nil {
		m.DeployCount.WithLabelValues(m.Hostname).Add(float64(n))
	}

	if m.StatsdAddr == "" {
		return
	}
	key := fmt.Sprintf(deployCount, m.Hostname)
	if err := m.count(key, n, 1.0); err != nil {
		log.Errorf("[SendDeployCount] Error occurred while counting: %v", err)
	}
}

// Client is a metrics obj
var Client = Metrics{}

// InitMetrics new a metrics obj
func InitMetrics(statsd string) error {
	hostname, err := os.Hostname()
	if err != nil {
		return err
	}
	Client = Metrics{StatsdAddr: statsd, Hostname: utils.CleanStatsdMetrics(hostname)}

	Client.MemoryCapacity = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "memory_capacity",
		Help: "node available memory.",
	}, []string{"nodename"})

	Client.DeployCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "core_deploy",
		Help: "core deploy counter",
	}, []string{"hostname"})

	Client.CPUMap = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "cpu_map",
		Help: "node available cpu.",
	}, []string{"nodename", "cpuid"})

	prometheus.MustRegister(Client.DeployCount, Client.MemoryCapacity, Client.CPUMap)
	return nil
}
