package types

import (
	"net"
	"net/url"
	"sync"
	"time"

	engineapi "github.com/docker/engine-api/client"
	enginetypes "github.com/docker/engine-api/types"
	"golang.org/x/net/context"
)

type CPUMap map[string]int

type CPUAndMem struct {
	CpuMap CPUMap
	MemCap int64
}

// total quotas
func (c CPUMap) Total() int {
	count := 0
	for _, value := range c {
		count += value
	}
	return count
}

func (c CPUMap) Add(q CPUMap) {
	for label, value := range q {
		if _, ok := c[label]; !ok {
			c[label] = value
		} else {
			c[label] += value
		}
	}
}

func (c CPUMap) Sub(q CPUMap) {
	for label, value := range q {
		if _, ok := c[label]; ok {
			c[label] -= value
		}
	}
}

type Node struct {
	sync.Mutex

	Name      string            `json:"name"`
	Endpoint  string            `json:"endpoint"`
	Podname   string            `json:"podname"`
	Public    bool              `json:"public"`
	Available bool              `json:"available"`
	CPU       CPUMap            `json:"cpu"`
	MemCap    int64             `json:"memcap"`
	Engine    *engineapi.Client `json:"-"`
}

// 2 seconds timeout
// used to be 5, but client won't wait that long
func (n *Node) Info() (enginetypes.Info, error) {
	ctx, _ := context.WithTimeout(context.Background(), 2*time.Second)
	return n.Engine.Info(ctx)
}

// get IP for node
// will not return error
func (n *Node) GetIP() string {
	u, err := url.Parse(n.Endpoint)
	if err != nil {
		return ""
	}

	host, _, err := net.SplitHostPort(u.Host)
	if err != nil {
		return ""
	}

	return host
}
