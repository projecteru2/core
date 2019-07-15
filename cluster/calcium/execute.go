package calcium

import (
	"context"

	"github.com/projecteru2/core/types"
)

// ExecuteContainer executes commands in running containers
func (c *Calcium) ExecuteContainer(ctx context.Context, opts *types.ExecuteContainerOptions) (ch <-chan *types.ExecuteContainerMessage, err error) {
	return
}
