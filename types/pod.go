package types

import (
	"sync"
)

type Pod struct {
	sync.Mutex
	Name string `json:"name"`
	Desc string `json:"desc"`
}
