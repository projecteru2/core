package calcium

import (
	"context"
	"strings"

	"github.com/projecteru2/core/types"
)

func (c *calcium) DeployStatusStream(ctx context.Context, appname, entrypoint, nodename string) chan *types.DeployStatus {
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

func parseStatusKey(key string) (string, string, string, string) {
	parts := strings.Split(key, "/")
	l := len(parts)
	return parts[l-4], parts[l-3], parts[l-2], parts[l-1]
}
