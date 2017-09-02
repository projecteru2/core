package stats

import (
	"fmt"
	"os"
	"strings"

	"gitlab.ricebook.net/platform/core/types"

	statsdlib "github.com/CMGS/statsd"
	log "github.com/Sirupsen/logrus"
)

const (
	memStats    = "eru-core.%s.%s.%%s"
	deployCount = "eru-core.deploy.count"

	beforeKey = "before"
	afterKey  = "after"
)

type statsdClient struct {
	Addr     string
	Hostname string
}

func (s *statsdClient) gauge(keyPattern string, data map[string]float64) error {
	remote, err := statsdlib.New(s.Addr)
	if err != nil {
		log.Errorf("[gauge] Connect statsd failed: %v", err)
		return err
	}
	defer remote.Close()
	defer remote.Flush()
	for k, v := range data {
		key := fmt.Sprintf(keyPattern, k)
		remote.Gauge(key, v)
	}
	return nil
}

func (s *statsdClient) count(key string, n int, rate float32) error {
	remote, err := statsdlib.New(s.Addr)
	if err != nil {
		log.Errorf("[count] Connect statsd failed: %v", err)
		return err
	}
	defer remote.Close()
	defer remote.Flush()
	remote.Count(key, n, rate)
	return nil
}

func (s *statsdClient) isNotSet() bool {
	return s.Addr == ""
}

func (s *statsdClient) SendMemCap(cpumemmap map[string]types.CPUAndMem, before bool) {
	if s.isNotSet() {
		return
	}
	data := map[string]float64{}
	for node, cpuandmem := range cpumemmap {
		data[node] = float64(cpuandmem.MemCap)
	}

	keyPattern := fmt.Sprintf(memStats, s.Hostname, beforeKey)
	if !before {
		keyPattern = fmt.Sprintf(memStats, s.Hostname, afterKey)
	}

	if err := s.gauge(keyPattern, data); err != nil {
		log.Errorf("[SendMemCap] Error occured while sending data to statsd: %v", err)
	}
}

func (s *statsdClient) SendDeployCount(n int) {
	if s.isNotSet() {
		return
	}
	if err := s.count(deployCount, n, 1.0); err != nil {
		log.Errorf("[SendDeployCount] Error occured while counting: %v", err)
	}
}

var Client = statsdClient{}

func NewStatsdClient(addr string) {
	hostname, _ := os.Hostname()
	cleanHost := strings.Replace(hostname, ".", "-", -1)
	Client = statsdClient{addr, cleanHost}
}
