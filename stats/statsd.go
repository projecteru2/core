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
	MEMSTATS = "eru-core.%s.%s.%s"

	BEFORE = "before"
	AFTER  = "after"
)

type statsdClient struct {
	Addr     string
	Hostname string
}

func (s *statsdClient) Close() error {
	return nil
}

func (s *statsdClient) send(data map[string]float64, endpoint, tag string) error {
	remote, err := statsdlib.New(s.Addr)
	if err != nil {
		log.Errorf("Connect statsd failed: %v", err)
		return err
	}
	defer remote.Close()
	defer remote.Flush()
	for k, v := range data {
		key := fmt.Sprintf(MEMSTATS, endpoint, tag, k)
		remote.Gauge(key, v)
	}
	return nil
}

func (s *statsdClient) SendMemCap(cpumemmap map[string]types.CPUAndMem, before bool) {
	data := map[string]float64{}
	for node, cpuandmem := range cpumemmap {
		data[node] = float64(cpuandmem.MemCap)
	}

	cleanHost := strings.Replace(s.Hostname, ".", "-", -1)
	tag := BEFORE
	if !before {
		tag = AFTER
	}
	err := s.send(data, cleanHost, tag)
	if err != nil {
		log.Errorf("Error occured while sending data to statsd: %v", err)
	}
}

var Client = statsdClient{}

func NewStatsdClient(addr string) {
	hostname, _ := os.Hostname()
	Client = statsdClient{addr, hostname}
}
