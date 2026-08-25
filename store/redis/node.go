package redis

import (
	"context"
	"encoding/json"
	"path/filepath"
	"time"

	"github.com/projecteru2/core/store/common"
	"github.com/projecteru2/core/types"
)

func (r *Rediaron) SetNodeStatus(ctx context.Context, node *types.Node, ttl int64) error {
	if ttl == 0 {
		return types.ErrInvaildNodeStatusTTL
	}

	key := filepath.Join(common.NodeStatusPrefix, node.Name)
	if ttl < 0 {
		_, err := r.cli.Del(ctx, key).Result()
		return err
	}

	data, err := json.Marshal(types.NodeStatus{
		Nodename: node.Name,
		Podname:  node.Podname,
		Alive:    true,
	})
	if err != nil {
		return err
	}

	_, err = r.cli.Set(ctx, key, string(data), time.Duration(ttl)*time.Second).Result()
	return err
}
