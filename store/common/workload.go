package common

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/projecteru2/core/types"
)

type StatusBinder interface {
	BindStatus(ctx context.Context, entityKey, statusKey, statusValue string, ttl int64) error
}

func SetWorkloadStatus(ctx context.Context, binder StatusBinder, status *types.StatusMeta, ttl int64) error {
	if status.Appname == "" || status.Entrypoint == "" || status.Nodename == "" {
		return types.ErrInvaildWorkloadStatus
	}

	data, err := json.Marshal(status)
	if err != nil {
		return err
	}
	statusKey := filepath.Join(WorkloadStatusPrefix, status.Appname, status.Entrypoint, status.Nodename, status.ID)
	workloadKey := fmt.Sprintf(WorkloadInfoKey, status.ID)
	return binder.BindStatus(ctx, workloadKey, statusKey, string(data), ttl)
}
