package utils

import (
	"github.com/panjf2000/ants/v2"
)

// NewPool new a pool
func NewPool(max int) (*ants.PoolWithFunc, error) {
	return ants.NewPoolWithFunc(max, func(i interface{}) {
		defer SentryDefer()
		f, _ := i.(func())
		f()
	}, ants.WithNonblocking(true))
}
