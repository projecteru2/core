package utils

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	enginemocks "github.com/projecteru2/core/engine/mocks"
)

func TestCache(t *testing.T) {
	c := NewEngineCache(2*time.Second, time.Second)

	host := "1.1.1.1"
	cli := &enginemocks.API{}
	c.Set(host, cli)
	assert.Equal(t, c.Get(host), cli)
	c.Delete(host)
	assert.Nil(t, c.Get(host))
	time.Sleep(3 * time.Second)
	assert.Nil(t, c.Get(host))
}
