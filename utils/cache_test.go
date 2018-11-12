package utils

import (
	"testing"

	engineapi "github.com/docker/docker/client"
	"github.com/stretchr/testify/assert"
)

func TestCache(t *testing.T) {
	c := Cache{
		Clients: map[string]*engineapi.Client{},
	}

	host := "1.1.1.1"
	cli := &engineapi.Client{}
	c.Set(host, cli)
	assert.Equal(t, c.Get(host), cli)
	c.Delete(host)
	assert.Nil(t, c.Get(host))
}
