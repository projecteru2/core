package discovery

import (
	"context"

	"github.com/projecteru2/core/types"

	"github.com/google/uuid"
)

// Service .
type Service interface {
	Subscribe(ctx context.Context) (uuid.UUID, <-chan types.ServiceStatus)
	Unsubscribe(ID uuid.UUID)
}
