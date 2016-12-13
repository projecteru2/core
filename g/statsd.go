package g

import (
	"fmt"

	statsdlib "github.com/CMGS/statsd"
	log "github.com/Sirupsen/logrus"
)

type StatsDClient struct {
	Addr string
}

func (self *StatsDClient) Close() error {
	return nil
}

func (self *StatsDClient) Send(data map[string]float64, endpoint, tag string) error {
	remote, err := statsdlib.New(self.Addr)
	if err != nil {
		log.Errorf("Connect statsd failed", err)
		return err
	}
	defer remote.Close()
	defer remote.Flush()
	for k, v := range data {
		key := fmt.Sprintf("eru-core.%s.%s.%s", endpoint, tag, k)
		remote.Gauge(key, v)
	}
	return nil
}

var Statsd = StatsDClient{}

func NewStatsdClient(addr string) {
	Statsd = StatsDClient{addr}
}
