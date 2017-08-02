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
	MEMSTATS    = "eru-core.%s.%s.%%s"
	DEPLOYCOUNT = "eru-core.deploy.count"

	BEFORE = "before"
	AFTER  = "after"
)

type statsdClient struct {
	Addr     string
	Hostname string
}

func (s *statsdClient) gauge(keyPattern string, data map[string]float64) error {
	remote, err := statsdlib.New(s.Addr)
	if err != nil {
		log.Errorf("Connect statsd failed: %v", err)
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
		log.Errorf("Connect statsd failed: %v", err)
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

	keyPattern := fmt.Sprintf(MEMSTATS, s.Hostname, BEFORE)
	if !before {
		keyPattern = fmt.Sprintf(MEMSTATS, s.Hostname, AFTER)
	}

	if err := s.gauge(keyPattern, data); err != nil {
		log.Errorf("Error occured while sending data to statsd: %v", err)
	}
}

func (s *statsdClient) SendDeployCount(n int) {
	if s.isNotSet() {
		return
	}
	if err := s.count(DEPLOYCOUNT, n, 1.0); err != nil {
		log.Errorf("Error occured while counting: %v", err)
	}
}

var Client = statsdClient{}

func NewStatsdClient(addr string) {
	hostname, _ := os.Hostname()
	cleanHost := strings.Replace(hostname, ".", "-", -1)
	Client = statsdClient{addr, cleanHost}
}
