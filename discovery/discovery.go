package discovery

import "github.com/projecteru2/core/types"

type Service interface {
	Subscribe() (uint32, <-chan types.ServiceStatus)
	Unsubscribe(ID uint32)
}
