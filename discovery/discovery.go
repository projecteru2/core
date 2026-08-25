package discovery

import (
	"context"

	"github.com/projecteru2/core/types"
)

type Service interface {
	Subscribe(ctx context.Context) (uint32, <-chan types.ServiceStatus)
	Unsubscribe(ID uint32)
}
