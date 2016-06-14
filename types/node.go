package types

import (
	"sync"

	engineapi "github.com/docker/engine-api/client"
	enginetypes "github.com/docker/engine-api/types"
	"golang.org/x/net/context"
)

type Node struct {
	sync.Mutex

	Name     string            `json:"name"`
	Endpoint string            `json:"endpoint"`
	Podname  string            `json:"podname"`
	Public   bool              `json:"public"`
	Cores    map[string]int    `json:"cores"`
	Engine   *engineapi.Client `json:"-"`
}

func (n *Node) Info() (enginetypes.Info, error) {
	return n.Engine.Info(context.Background())
}
