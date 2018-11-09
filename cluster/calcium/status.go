package calcium

import (
	"context"

	"github.com/projecteru2/core/types"
)

// DeployStatusStream watch deploy status
func (c *Calcium) DeployStatusStream(ctx context.Context, appname, entrypoint, nodename string) chan *types.DeployStatus {
	return c.store.WatchDeployStatus(ctx, appname, entrypoint, nodename)
}
