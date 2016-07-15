package types

import (
	"sync"

	engineapi "github.com/docker/engine-api/client"
	enginetypes "github.com/docker/engine-api/types"
	"golang.org/x/net/context"
)

type CPUMap map[string]int

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

	Name     string            `json:"name"`
	Endpoint string            `json:"endpoint"`
	Podname  string            `json:"podname"`
	Public   bool              `json:"public"`
	CPU      CPUMap            `json:"cpu"`
	Engine   *engineapi.Client `json:"-"`
}

func (n *Node) Info() (enginetypes.Info, error) {
	return n.Engine.Info(context.Background())
}
