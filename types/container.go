package types

import (
	"context"
	"time"

	enginetypes "github.com/docker/docker/api/types"
	engineapi "github.com/docker/docker/client"
)

// Container store container info
// only relationship with pod and node is stored
// if you wanna get realtime information, use Inspect method
type Container struct {
	ID         string            `json:"id"`
	Podname    string            `json:"podname"`
	Nodename   string            `json:"nodename"`
	Name       string            `json:"name"`
	CPU        CPUMap            `json:"cpu"`
	Quota      float64           `json:"quota"`
	Memory     int64             `json:"memory"`
	Hook       *Hook             `json:"hook"`
	Privileged bool              `json:"privileged"`
	SoftLimit  bool              `json:"softlimit"`
	Engine     *engineapi.Client `json:"-"`
	Node       *Node             `json:"-"`
}

// Inspect a container
func (c *Container) Inspect(ctx context.Context) (enginetypes.ContainerJSON, error) {
	inspectCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if c.Engine == nil {
		return enginetypes.ContainerJSON{}, ErrNilEngine
	}
	return c.Engine.ContainerInspect(inspectCtx, c.ID)
}

// Stop a container
func (c *Container) Stop(ctx context.Context, timeout time.Duration) error {
	if c.Engine == nil {
		return ErrNilEngine
	}
	// 这里 block 的问题很严重，按照目前的配置是 5 分钟一级的 block
	// 一个简单的处理方法是相信 ctx 不相信 docker 自身的处理
	// 另外我怀疑 docker 自己的 timeout 实现是完全的等 timeout 而非结束了就退出
	removeCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return c.Engine.ContainerStop(removeCtx, c.ID, nil)
}

// Remove a container
func (c *Container) Remove(ctx context.Context) error {
	if c.Engine == nil {
		return ErrNilEngine
	}
	rmOpts := enginetypes.ContainerRemoveOptions{
		RemoveVolumes: true,
		Force:         true,
	}
	return c.Engine.ContainerRemove(ctx, c.ID, rmOpts)
}

// DeployStatus store deploy status
type DeployStatus struct {
	Data       string
	Err        error
	Action     string
	Appname    string
	Entrypoint string
	Nodename   string
	ID         string
}

// ContainerMeta bind meta info store in labels
type ContainerMeta struct {
	Publish     []string
	HealthCheck *HealthCheck
}
