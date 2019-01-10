package types

import (
	"context"

	engine "github.com/projecteru2/core/engine"
	enginetypes "github.com/projecteru2/core/engine/types"
)

const (
	// IncrUsage add cpuusage
	IncrUsage = "+"
	// DecrUsage cpuusage
	DecrUsage = "-"
)

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

// Map return cpu map
func (c CPUMap) Map() map[string]int {
	return map[string]int(c)
}

// Node store node info
type Node struct {
	Name       string            `json:"name"`
	Endpoint   string            `json:"endpoint"`
	Podname    string            `json:"podname"`
	CPU        CPUMap            `json:"cpu"`
	CPUUsed    float64           `json:"cpuused"`
	MemCap     int64             `json:"memcap"`
	Available  bool              `json:"available"`
	Labels     map[string]string `json:"labels"`
	InitCPU    CPUMap            `json:"init_cpu"`
	InitMemCap int64             `json:"init_memcap"`
	Engine     engine.API        `json:"-"`
}

// Info show node info
func (n *Node) Info(ctx context.Context) (*enginetypes.Info, error) {
	if n.Engine == nil {
		return nil, ErrNilEngine
	}
	return n.Engine.Info(ctx)
}

// SetCPUUsed set cpuusage
func (n *Node) SetCPUUsed(quota float64, action string) {
	switch action {
	case IncrUsage:
		n.CPUUsed = Round(n.CPUUsed + quota)
	case DecrUsage:
		n.CPUUsed = Round(n.CPUUsed - quota)
	default:
	}
}

// NodeInfo for deploy
type NodeInfo struct {
	Name     string
	CPUMap   CPUMap
	MemCap   int64
	CPUUsed  float64 // CPU目前占用率
	MemUsage float64 // MEM目前占用率
	CPURate  float64 // 需要增加的 CPU 占用率
	MemRate  float64 // 需要增加的内存占有率

	CPUPlan  []CPUMap
	Capacity int // 可以部署几个
	Count    int // 上面有几个了
	Deploy   int // 最终部署几个
	// 其他需要 filter 的字段
}
