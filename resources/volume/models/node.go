package models

import (
	"context"
	"fmt"

	"github.com/cockroachdb/errors"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/resources/volume/types"
	coretypes "github.com/projecteru2/core/types"
)

// AddNode .
func (v *Volume) AddNode(ctx context.Context, node string, resourceOpts *types.NodeResourceOpts) (*types.NodeResourceInfo, error) {
	if _, err := v.doGetNodeResourceInfo(ctx, node); err != nil {
		if !errors.Is(err, coretypes.ErrInvaildCount) {
			log.WithFunc("resources.volume.AddNode").WithField("node", node).Error(ctx, err, "failed to get resource info of node")
			return nil, err
		}
	} else {
		return nil, coretypes.ErrNodeExists
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
		log.WithFunc("resources.volume.RemoveNode").WithField("node", node).Errorf(ctx, err, "faield to delete node")
		return err
	}
	return nil
}
