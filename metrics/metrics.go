package metrics

import (
	"context"
	"fmt"
	"os"

	statsdlib "github.com/CMGS/statsd"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

const (
	cpuMap       = "core.node.%s.cpu.%s"
	memStats     = "core.node.%s.memory"
	storageStats = "core.node.%s.storage"
	deployCount  = "core.%s.deploy.count"
)

// Metrics define metrics
type Metrics struct {
	StatsdAddr   string
	Hostname     string
	statsdClient *statsdlib.Client

	PodAllocatedMemory *prometheus.GaugeVec
	PodInitMemory      *prometheus.GaugeVec

	PodAllocatedCPU *prometheus.GaugeVec
	PodInitCPU      *prometheus.GaugeVec

	PodAllocatedStorage *prometheus.GaugeVec
	PodInitStorage      *prometheus.GaugeVec

	MemoryCapacity  *prometheus.GaugeVec
	StorageCapacity *prometheus.GaugeVec
	CPUMap          *prometheus.GaugeVec
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
	log.Debugf("[Metrics] Update %s metrics", node.Name)
	nodename := utils.CleanStatsdMetrics(node.Name)
	podname := utils.CleanStatsdMetrics(node.Podname)
	memory := float64(node.MemCap)
	storage := float64(node.StorageCap)

	if m.MemoryCapacity != nil {
		m.MemoryCapacity.WithLabelValues(podname, nodename).Set(memory)
	}

	if m.StorageCapacity != nil {
		m.StorageCapacity.WithLabelValues(podname, nodename).Set(storage)
	}

	for cpuid, value := range node.CPU {
		val := float64(value)

		if m.CPUMap != nil {
			m.CPUMap.WithLabelValues(podname, nodename, cpuid).Set(val)
		}

		if m.StatsdAddr == "" {
			continue
		}

		if err := m.gauge(fmt.Sprintf(cpuMap, nodename, cpuid), val); err != nil {
			log.Errorf("[SendNodeInfo] Error occurred while sending cpu data to statsd: %v", err)
		}
	}

	if m.StatsdAddr == "" {
		return
	}

	if err := m.gauge(fmt.Sprintf(memStats, nodename), memory); err != nil {
		log.Errorf("[SendNodeInfo] Error occurred while sending memory data to statsd: %v", err)
	}

	if err := m.gauge(fmt.Sprintf(storageStats, nodename), storage); err != nil {
		log.Errorf("[SendNodeInfo] Error occurred while sending storage data to statsd: %v", err)
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

var needInitNodeStatus = true

// UpdatePodMetrics ...
func (m *Metrics) UpdatePodMetrics(ctx context.Context, pod *types.Pod, listOfNodes []*types.Node) {
	initMemory := float64(0)
	allocatedMemory := float64(0)
	initCPU := float64(0)
	allocatedCPU := float64(0)
	initStorage := float64(0)
	allocatedStorage := float64(0)

	for _, node := range listOfNodes {
		if needInitNodeStatus {
			m.SendNodeInfo(node)
		}
		initMemory += float64(node.InitMemCap)
		allocatedMemory += float64(node.InitMemCap - node.MemCap)

		initCPU += float64(len(node.InitCPU))
		allocatedCPU += node.CPUUsed

		initStorage += float64(node.InitStorageCap)
		allocatedStorage += float64(node.InitStorageCap - node.StorageCap)
	}
	needInitNodeStatus = false

	if m.PodInitMemory != nil && m.PodAllocatedMemory != nil {
		m.PodInitMemory.WithLabelValues(pod.Name).Set(initMemory)
		m.PodAllocatedMemory.WithLabelValues(pod.Name).Set(allocatedMemory)
	}

	if m.PodInitCPU != nil && m.PodAllocatedCPU != nil {
		m.PodInitCPU.WithLabelValues(pod.Name).Set(initCPU)
		m.PodAllocatedCPU.WithLabelValues(pod.Name).Set(allocatedCPU)

	}

	if m.PodInitStorage != nil && m.PodAllocatedStorage != nil {
		m.PodInitStorage.WithLabelValues(pod.Name).Set(initStorage)
		m.PodAllocatedStorage.WithLabelValues(pod.Name).Set(allocatedStorage)
	}
}

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
	}, []string{"podname", "nodename"})

	Client.StorageCapacity = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "storage_capacity",
		Help: "node available storage.",
	}, []string{"podname", "nodename"})

	Client.CPUMap = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "cpu_map",
		Help: "node available cpu.",
	}, []string{"podname", "nodename", "cpuid"})

	Client.DeployCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "core_deploy",
		Help: "core deploy counter",
	}, []string{"hostname"})

	Client.PodInitMemory = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "pod_init_memory",
		Help: "pod init memory.",
	}, []string{"podname"})

	Client.PodAllocatedMemory = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "pod_allocated_memory",
		Help: "pod allocated memory.",
	}, []string{"podname"})

	Client.PodInitCPU = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "pod_init_cpu",
		Help: "pod init cpu.",
	}, []string{"podname"})

	Client.PodAllocatedCPU = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "pod_allocated_cpu",
		Help: "pod allocated cpu.",
	}, []string{"podname"})

	Client.PodInitStorage = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "pod_init_storage",
		Help: "pod init storage.",
	}, []string{"podname"})

	Client.PodAllocatedStorage = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "pod_allocated_storage",
		Help: "pod allocated storage.",
	}, []string{"podname"})

	prometheus.MustRegister(
		Client.DeployCount, Client.MemoryCapacity,
		Client.StorageCapacity, Client.CPUMap,
		Client.PodAllocatedMemory, Client.PodInitMemory,
		Client.PodAllocatedCPU, Client.PodInitCPU,
		Client.PodAllocatedStorage, Client.PodInitStorage,
	)
	return nil
}
