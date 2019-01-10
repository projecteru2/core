package calico

import (
	"context"
	"fmt"

	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
	log "github.com/sirupsen/logrus"
)

// Titanium for calico
type Titanium struct{}

// ConnectToNetwork to network with ipv4 address
func (t *Titanium) ConnectToNetwork(ctx context.Context, containerID, networkID, ipv4 string) error {
	if len(containerID) != 64 {
		return types.ErrBadContainerID
	}

	engine, ok := utils.GetDockerEngineFromContext(ctx)
	if !ok {
		return types.NewDetailedErr(types.ErrCannotGetEngine,
			fmt.Sprintf("Not actually a `engine.API` for value engine in context"))
	}

	return engine.NetworkConnect(ctx, networkID, containerID, ipv4, "")
}

// DisconnectFromNetwork from network
func (t *Titanium) DisconnectFromNetwork(ctx context.Context, containerID, networkID string) error {
	if len(containerID) != 64 {
		return types.ErrBadContainerID
	}

	engine, ok := utils.GetDockerEngineFromContext(ctx)
	if !ok {
		return types.NewDetailedErr(types.ErrCannotGetEngine,
			fmt.Sprintf("Not actually a `engine.API` for value engine in context"))
	}

	log.Infof("[DisconnectFromNetwork] Disconnect %v from %v", containerID, networkID)
	return engine.NetworkDisconnect(ctx, networkID, containerID, false)
}

// ListNetworks networks from context
func (t *Titanium) ListNetworks(ctx context.Context, driver string) ([]*enginetypes.Network, error) {
	engine, ok := utils.GetDockerEngineFromContext(ctx)
	if !ok {
		return nil, types.NewDetailedErr(types.ErrCannotGetEngine,
			fmt.Sprintf("Not actually a `engine.API` for value engine in context"))
	}

	return engine.NetworkList(ctx, []string{driver})
}

// New a titanium obj
func New() *Titanium {
	return &Titanium{}
}
