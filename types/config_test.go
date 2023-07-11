package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIdentifier(t *testing.T) {
	config := Config{}
	config.Etcd = EtcdConfig{
		Machines: []string{
			"1.1.1.1",
			"2.2.2.2",
		},
	}
	r, err := config.Identifier()
	assert.NoError(t, err)
	assert.NotEmpty(t, r)
}
