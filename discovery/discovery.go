package discovery

import (
	"github.com/projecteru2/core/types"

	"github.com/google/uuid"
)

// Service .
type Service interface {
	Subscribe(ch chan<- types.ServiceStatus) uuid.UUID
	Unsubscribe(id uuid.UUID)
}
