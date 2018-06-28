package calcium

import (
	"context"

	"github.com/projecteru2/core/types"
)

//DeployStatusStream watch deploy status
func (c *Calcium) DeployStatusStream(ctx context.Context, appname, entrypoint, nodename string) chan *types.DeployStatus {
	ch := make(chan *types.DeployStatus)
	watcher := c.store.WatchDeployStatus(appname, entrypoint, nodename)
	go func() {
		defer close(ch)
		for {
			info, err := watcher.Next(ctx)
			msg := &types.DeployStatus{}
			if err != nil {
				if err != context.Canceled {
					msg.Err = err
					ch <- msg
				}
				return
			}
			appname, entrypoint, nodename, id := parseStatusKey(info.Node.Key)
			msg.Data = info.Node.Value
			msg.Action = info.Action
			msg.Appname = appname
			msg.Entrypoint = entrypoint
			msg.Nodename = nodename
			msg.ID = id
			ch <- msg
		}
	}()
	return ch
}
