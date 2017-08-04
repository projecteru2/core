package types

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"sync"
	"time"

	enginetypes "github.com/docker/docker/api/types"
	engineapi "github.com/docker/docker/client"
)

type CPUAndMem struct {
	CpuMap CPUMap
	MemCap int64
}

// CPUMap {["0"]10000, ["1"]10000}
type CPUMap map[string]int64

// Total quotas
func (c CPUMap) Total() int64 {
	var count int64
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
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if n.Engine == nil {
		return enginetypes.Info{}, fmt.Errorf("Node engine is nil")
	}
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

type NodeInfo struct {
	CPUAndMem
	Name     string
	CPURate  int64
	Capacity int // 可以部署几个
	Count    int // 上面有几个了
	Deploy   int // 最终部署几个
	// 其他需要 filter 的字段
}
