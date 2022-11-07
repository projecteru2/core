package utils

import (
	"github.com/panjf2000/ants/v2"
	"github.com/projecteru2/core/log"
)

// NewPool new a pool
func NewPool(max int) (*ants.PoolWithFunc, error) {
	return ants.NewPoolWithFunc(max, func(i interface{}) {
		defer log.SentryDefer()
		f, _ := i.(func())
		f()
	}, ants.WithNonblocking(true))
}
