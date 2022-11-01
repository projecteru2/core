package models

import (
	"context"
	"fmt"

	"github.com/pkg/errors"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/resources/volume/types"
	coretypes "github.com/projecteru2/core/types"
)

// AddNode .
func (v *Volume) AddNode(ctx context.Context, node string, resourceOpts *types.NodeResourceOpts) (*types.NodeResourceInfo, error) {
	if _, err := v.doGetNodeResourceInfo(ctx, node); err != nil {
		if !errors.Is(err, coretypes.ErrBadCount) {
			log.Errorf(ctx, err, "[AddNode] failed to get resource info of node %+v", node)
			return nil, err
		}
	} else {
		return nil, types.ErrNodeExists
	}

	resourceInfo := &types.NodeResourceInfo{
		Capacity: &types.NodeResourceArgs{
			Volumes: resourceOpts.Volumes,
			Storage: resourceOpts.Storage,
			Disks:   resourceOpts.Disks,
		},
		Usage: nil,
	}

	return resourceInfo, v.doSetNodeResourceInfo(ctx, node, resourceInfo)
}

// RemoveNode .
func (v *Volume) RemoveNode(ctx context.Context, node string) error {
	if _, err := v.store.Delete(ctx, fmt.Sprintf(NodeResourceInfoKey, node)); err != nil {
		log.Errorf(ctx, err, "[RemoveNode] faield to delete node %+v", node)
		return err
	}
	return nil
}
