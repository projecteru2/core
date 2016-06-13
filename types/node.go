package types

import (
	"net/http"
	"sync"

	"github.com/docker/engine-api/client"
)

type Node struct {
	sync.Mutex

	Name     string         `json:"name"`
	Endpoint string         `json:"endpoint"`
	Podname  string         `json:"podname"`
	Public   bool           `json:"public"`
	Cores    map[string]int `json:"cores"`
	Engine   *client.Client `json:"-"`
}
