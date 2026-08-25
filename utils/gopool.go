package utils

import (
	"github.com/panjf2000/ants/v2"

	"github.com/projecteru2/core/log"
)

// NewPool returns a blocking pool; a full pool makes Invoke wait for a free worker.
func NewPool(max int) (*ants.PoolWithFunc, error) {
	return ants.NewPoolWithFunc(max, func(i any) {
		defer log.SentryDefer()
		f, _ := i.(func())
		f()
	})
}
