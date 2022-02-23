package metrics

import (
	"fmt"
	"os"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"

	statsdlib "github.com/CMGS/statsd"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	//cpuMap           = "core.node.%s.cpu.%s"
	//memStats         = "core.node.%s.memory"
	//storageStats     = "core.node.%s.storage"
	//memUsedStats     = "core.node.%s.memory.used"
	//storageUsedStats = "core.node.%s.storage.used"
	//cpuUsedStats     = "core.node.%s.cpu.used"
	deployCount = "core.%s.deploy.count"
)

// Metrics define metrics
type Metrics struct {
	Config types.Config

	StatsdAddr   string
	Hostname     string
	statsdClient *statsdlib.Client

	MemoryCapacity  *prometheus.GaugeVec
	MemoryUsed      *prometheus.GaugeVec
	StorageCapacity *prometheus.GaugeVec
	StorageUsed     *prometheus.GaugeVec
	CPUMap          *prometheus.GaugeVec
	CPUUsed         *prometheus.GaugeVec
	DeployCount     *prometheus.CounterVec
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

//func (m *Metrics) gauge(key string, value float64) error {
//	if err := m.checkConn(); err != nil {
//		return err
//	}
//	m.statsdClient.Gauge(key, value)
//	return nil
//}
//
func (m *Metrics) count(key string, n int, rate float32) error {
	if err := m.checkConn(); err != nil {
		return err
	}
	m.statsdClient.Count(key, n, rate)
	return nil
}

//
//// SendNodeInfo update node resource capacity
//func (m *Metrics) SendNodeInfo(nm *types.NodeMetrics) {
//	nodename := nm.Name
//	podname := nm.Podname
//	memory := nm.Memory
//	memoryUsed := nm.MemoryUsed
//	storage := nm.Storage
//	storageUsed := nm.StorageUsed
//	cpuUsed := nm.CPUUsed
//
//	if m.MemoryCapacity != nil {
//		m.MemoryCapacity.WithLabelValues(podname, nodename).Set(memory)
//	}
//
//	if m.MemoryUsed != nil {
//		m.MemoryUsed.WithLabelValues(podname, nodename).Set(memoryUsed)
//	}
//
//	if m.StorageCapacity != nil {
//		m.StorageCapacity.WithLabelValues(podname, nodename).Set(storage)
//	}
//
//	if m.StorageUsed != nil {
//		m.StorageUsed.WithLabelValues(podname, nodename).Set(storageUsed)
//	}
//
//	if m.CPUUsed != nil {
//		m.CPUUsed.WithLabelValues(podname, nodename).Set(cpuUsed)
//	}
//
//	cleanedNodeName := utils.CleanStatsdMetrics(nodename)
//	for cpuid, value := range nm.CPU {
//		val := float64(value)
//
//		if m.CPUMap != nil {
//			m.CPUMap.WithLabelValues(podname, nodename, cpuid).Set(val)
//		}
//
//		if m.StatsdAddr == "" {
//			continue
//		}
//
//		if err := m.gauge(fmt.Sprintf(cpuMap, cleanedNodeName, cpuid), val); err != nil {
//			log.Errorf(nil, "[SendNodeInfo] Error occurred while sending cpu data to statsd: %v", err) //nolint
//		}
//	}
//
//	if m.StatsdAddr == "" {
//		return
//	}
//
//	if err := m.gauge(fmt.Sprintf(memStats, cleanedNodeName), memory); err != nil {
//		log.Errorf(nil, "[SendNodeInfo] Error occurred while sending memory data to statsd: %v", err) //nolint
//	}
//
//	if err := m.gauge(fmt.Sprintf(storageStats, cleanedNodeName), storage); err != nil {
//		log.Errorf(nil, "[SendNodeInfo] Error occurred while sending storage data to statsd: %v", err) //nolint
//	}
//
//	if err := m.gauge(fmt.Sprintf(memUsedStats, cleanedNodeName), memoryUsed); err != nil {
//		log.Errorf(nil, "[SendNodeInfo] Error occurred while sending memory used data to statsd: %v", err) //nolint
//	}
//
//	if err := m.gauge(fmt.Sprintf(storageUsedStats, cleanedNodeName), storageUsed); err != nil {
//		log.Errorf(nil, "[SendNodeInfo] Error occurred while sending storage used data to statsd: %v", err) //nolint
//	}
//
//	if err := m.gauge(fmt.Sprintf(cpuUsedStats, cleanedNodeName), cpuUsed); err != nil {
//		log.Errorf(nil, "[SendNodeInfo] Error occurred while sending cpu used data to statsd: %v", err) //nolint
//	}
//}

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
		log.Errorf(nil, "[SendDeployCount] Error occurred while counting: %v", err) //nolint
	}
}

// Client is a metrics obj
var Client = Metrics{}

// InitMetrics new a metrics obj
func InitMetrics(config types.Config) error {
	hostname, err := os.Hostname()
	if err != nil {
		return err
	}
	Client = Metrics{
		Config:     config,
		StatsdAddr: config.Statsd,
		Hostname:   utils.CleanStatsdMetrics(hostname),
	}

	Client.MemoryCapacity = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "memory_capacity",
		Help: "node available memory.",
	}, []string{"podname", "nodename"})

	Client.MemoryUsed = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "memory_used",
		Help: "node used memory.",
	}, []string{"podname", "nodename"})

	Client.StorageCapacity = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "storage_capacity",
		Help: "node available storage.",
	}, []string{"podname", "nodename"})

	Client.StorageUsed = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "storage_used",
		Help: "node used storage.",
	}, []string{"podname", "nodename"})

	Client.CPUMap = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "cpu_map",
		Help: "node available cpu.",
	}, []string{"podname", "nodename", "cpuid"})

	Client.CPUUsed = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "cpu_used",
		Help: "node used cpu.",
	}, []string{"podname", "nodename"})

	Client.DeployCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "core_deploy",
		Help: "core deploy counter",
	}, []string{"hostname"})

	prometheus.MustRegister(
		Client.DeployCount, Client.MemoryCapacity,
		Client.StorageCapacity, Client.CPUMap,
		Client.MemoryUsed, Client.StorageUsed, Client.CPUUsed,
	)
	return nil
}
