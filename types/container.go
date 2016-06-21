package types

import (
	engineapi "github.com/docker/engine-api/client"
	enginetypes "github.com/docker/engine-api/types"
	"golang.org/x/net/context"
)

// only relationship with pod and node is stored
// if you wanna get realtime information, use Inspect method
type Container struct {
	ID       string            `json:"id"`
	Podname  string            `json:"podname"`
	Nodename string            `json:"nodename"`
	Name     string            `json:"name"`
	CPU      CPUMap            `json:"cpu"`
	Engine   *engineapi.Client `json:"-"`
}

func (c *Container) Inspect() (enginetypes.ContainerJSON, error) {
	return c.Engine.ContainerInspect(context.Background(), c.ID)
}
