package types

import (
	"context"
	"time"

	engine "github.com/projecteru2/core/engine"
	enginetypes "github.com/projecteru2/core/engine/types"
)

// Container store container info
// only relationship with pod and node is stored
// if you wanna get realtime information, use Inspect method
type Container struct {
	ID         string     `json:"id"`
	Podname    string     `json:"podname"`
	Nodename   string     `json:"nodename"`
	Name       string     `json:"name"`
	CPU        CPUMap     `json:"cpu"`
	Quota      float64    `json:"quota"`
	Memory     int64      `json:"memory"`
	Hook       *Hook      `json:"hook"`
	Privileged bool       `json:"privileged"`
	SoftLimit  bool       `json:"softlimit"`
	StatusData []byte     `json:"-"`
	Engine     engine.API `json:"-"`
}

// Inspect a container
func (c *Container) Inspect(ctx context.Context) (*enginetypes.VirtualizationInfo, error) {
	if c.Engine == nil {
		return nil, ErrNilEngine
	}
	inspectCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return c.Engine.VirtualizationInspect(inspectCtx, c.ID)
}

// Start a container
func (c *Container) Start(ctx context.Context) error {
	if c.Engine == nil {
		return ErrNilEngine
	}
	startCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	return c.Engine.VirtualizationStart(startCtx, c.ID)
}

// Stop a container
func (c *Container) Stop(ctx context.Context, timeout time.Duration) error {
	if c.Engine == nil {
		return ErrNilEngine
	}
	// 这里 block 的问题很严重，按照目前的配置是 5 分钟一级的 block
	// 一个简单的处理方法是相信 ctx 不相信 engine 自身的处理
	// 另外我怀疑 engine 自己的 timeout 实现是完全的等 timeout 而非结束了就退出
	removeCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return c.Engine.VirtualizationStop(removeCtx, c.ID)
}

// Remove a container
func (c *Container) Remove(ctx context.Context) error {
	if c.Engine == nil {
		return ErrNilEngine
	}
	return c.Engine.VirtualizationRemove(ctx, c.ID, true, true)
}

// DeployStatus store deploy status
type DeployStatus struct {
	Data       string
	Error      error
	Action     string
	Appname    string
	Entrypoint string
	Nodename   string
	ID         string
}

// EruMeta bind meta info store in labels
type EruMeta struct {
	Publish     []string
	HealthCheck *HealthCheck
}
