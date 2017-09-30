package types

import (
	"context"
	"fmt"
	"time"

	enginetypes "github.com/docker/docker/api/types"
	engineapi "github.com/docker/docker/client"
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
	Networks map[string]string `json:"networks"`
	Engine   *engineapi.Client `json:"-"`
}

func (c *Container) ShortID() string {
	containerID := c.ID
	if len(containerID) > 7 {
		return containerID[:7]
	}
	return containerID
}

func (c *Container) Inspect() (enginetypes.ContainerJSON, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if c.Engine == nil {
		return enginetypes.ContainerJSON{}, fmt.Errorf("Engine is nil")
	}
	return c.Engine.ContainerInspect(ctx, c.ID)
}
