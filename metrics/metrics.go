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
	memStats    = "eru.node.mem"
	deployCount = "eru.core.%s.deploy.count"
)

// Metrics define metrics
type Metrics struct {
	StatsdAddr string
	Hostname   string

	MemoryCapacity *prometheus.GaugeVec
	DeployCount    *prometheus.CounterVec
}

func (m *Metrics) gauge(keyPattern string, data map[string]float64) error {
	remote, err := statsdlib.New(m.StatsdAddr)
	if err != nil {
		log.Errorf("[gauge] Connect statsd failed: %v", err)
		return err
	}
	defer remote.Close()
	defer remote.Flush()
	for k, v := range data {
		key := fmt.Sprintf("%s.%s", keyPattern, k)
		remote.Gauge(key, v)
	}
	return nil
}

func (m *Metrics) count(key string, n int, rate float32) error {
	remote, err := statsdlib.New(m.StatsdAddr)
	if err != nil {
		log.Errorf("[count] Connect statsd failed: %v", err)
		return err
	}
	defer remote.Close()
	defer remote.Flush()
	remote.Count(key, n, rate)
	return nil
}

// SendMemCap update memory capacity
func (m *Metrics) SendMemCap(cpumemmap map[string]types.CPUAndMem) {
	log.Info("[Metrics] Update memory capacity gauge")
	data := map[string]float64{}
	for node, cpuandmem := range cpumemmap {
		nodename := utils.CleanStatsdMetrics(node)
		capacity := float64(cpuandmem.MemCap)
		data[nodename] = capacity
		if m.MemoryCapacity != nil {
			m.MemoryCapacity.WithLabelValues(nodename).Set(capacity)
		}
	}

	if m.StatsdAddr == "" {
		return
	}

	if err := m.gauge(memStats, data); err != nil {
		log.Errorf("[SendMemCap] Error occured while sending data to statsd: %v", err)
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
		log.Errorf("[SendDeployCount] Error occured while counting: %v", err)
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
		Help: "node avaliable memory.",
	}, []string{"nodename"})

	Client.DeployCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "core_deploy",
		Help: "core deploy counter",
	}, []string{"hostname"})

	prometheus.MustRegister(Client.DeployCount, Client.MemoryCapacity)
	return nil
}
