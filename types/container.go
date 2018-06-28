package types

import (
	"context"
	"fmt"
	"time"

	enginetypes "github.com/docker/docker/api/types"
	engineapi "github.com/docker/docker/client"
)

//Container store container info
// only relationship with pod and node is stored
// if you wanna get realtime information, use Inspect method
type Container struct {
	ID          string            `json:"id"`
	Podname     string            `json:"podname"`
	Nodename    string            `json:"nodename"`
	Name        string            `json:"name"`
	CPU         CPUMap            `json:"cpu"`
	Quota       float64           `json:"quota"`
	Memory      int64             `json:"memory"`
	Hook        *Hook             `json:"hook"`
	Privileged  bool              `json:"privileged"`
	RawResource bool              `json:"raw_resource"`
	Engine      *engineapi.Client `json:"-"`
}

//ShortID short container ID
func (c *Container) ShortID() string {
	containerID := c.ID
	if len(containerID) > 7 {
		return containerID[:7]
	}
	return containerID
}

//Inspect a container
func (c *Container) Inspect(ctx context.Context) (enginetypes.ContainerJSON, error) {
	inspectCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if c.Engine == nil {
		return enginetypes.ContainerJSON{}, fmt.Errorf("Engine is nil")
	}
	return c.Engine.ContainerInspect(inspectCtx, c.ID)
}

//DeployStatus store deploy status
type DeployStatus struct {
	Data       string
	Err        error
	Action     string
	Appname    string
	Entrypoint string
	Nodename   string
	ID         string
}
