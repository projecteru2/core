package discovery

import (
	"github.com/google/uuid"
	"github.com/projecteru2/core/types"
)

// Service .
type Service interface {
	Subscribe(ch chan<- types.ServiceStatus) uuid.UUID
	Unsubscribe(id uuid.UUID)
}
