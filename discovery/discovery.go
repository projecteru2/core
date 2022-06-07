package discovery

import (
	"context"

	"github.com/google/uuid"

	"github.com/projecteru2/core/types"
)

// Service .
type Service interface {
	Subscribe(ctx context.Context) (uuid.UUID, <-chan types.ServiceStatus)
	Unsubscribe(id uuid.UUID)
}
