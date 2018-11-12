package types

import (
	"context"
	"net"
	"net/url"
	"time"

	enginetypes "github.com/docker/docker/api/types"
	engineapi "github.com/docker/docker/client"
)

// CPUAndMem store cpu and mem
type CPUAndMem struct {
	CPUMap CPUMap
	MemCap int64
}

// CPUMap is cpu core map
// CPUMap {["0"]10000, ["1"]10000}
type CPUMap map[string]int

// Total show total cpu
// Total quotas
func (c CPUMap) Total() int {
	var count int
	for _, value := range c {
		count += value
	}
	return count
}

// Add return cpu
func (c CPUMap) Add(q CPUMap) {
	for label, value := range q {
		if _, ok := c[label]; !ok {
			c[label] = value
		} else {
			c[label] += value
		}
	}
}

// Sub decrease cpus
func (c CPUMap) Sub(q CPUMap) {
	for label, value := range q {
		if _, ok := c[label]; ok {
			c[label] -= value
		}
	}
}

// Node store node info
type Node struct {
	Name       string            `json:"name"`
	Endpoint   string            `json:"endpoint"`
	Podname    string            `json:"podname"`
	Available  bool              `json:"available"`
	CPU        CPUMap            `json:"cpu"`
	MemCap     int64             `json:"memcap"`
	Labels     map[string]string `json:"labels"`
	InitCPU    CPUMap            `json:"init_cpu"`
	InitMemCap int64             `json:"init_memcap"`
	Engine     *engineapi.Client `json:"-"`
}

// Info show node info
// 2 seconds timeout
// used to be 5, but client won't wait that long
func (n *Node) Info(ctx context.Context) (enginetypes.Info, error) {
	infoCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if n.Engine == nil {
		return enginetypes.Info{}, ErrNilEngine
	}
	return n.Engine.Info(infoCtx)
}

// GetIP get node ip
// get IP for node
// will not return error
func (n *Node) GetIP() string {
	host, _ := GetEndpointHost(n.Endpoint)
	return host
}

// NodeInfo for deploy
type NodeInfo struct {
	CPUAndMem
	Name     string
	CPUs     int
	CPUPlan  []CPUMap
	Capacity int // 可以部署几个
	Count    int // 上面有几个了
	Deploy   int // 最终部署几个
	// 其他需要 filter 的字段
}

// GetEndpointHost get host from endpoint
func GetEndpointHost(endpoint string) (string, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return "", err
	}

	host, _, err := net.SplitHostPort(u.Host)
	if err != nil {
		return "", err
	}
	return host, nil
}
