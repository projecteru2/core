package types

import (
	"time"

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
	Memory   int64             `json:"memory"`
	Engine   *engineapi.Client `json:"-"`
}

func (c *Container) Inspect() (enginetypes.ContainerJSON, error) {
	ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)
	return c.Engine.ContainerInspect(ctx, c.ID)
}
