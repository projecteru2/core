package utils

import (
	"testing"

	"github.com/projecteru2/core/engine"
	enginemocks "github.com/projecteru2/core/engine/mocks"
	"github.com/stretchr/testify/assert"
)

func TestCache(t *testing.T) {
	c := Cache{
		Clients: map[string]engine.API{},
	}

	host := "1.1.1.1"
	cli := &enginemocks.API{}
	c.Set(host, cli)
	assert.Equal(t, c.Get(host), cli)
	c.Delete(host)
	assert.Nil(t, c.Get(host))
}
