package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIdentifierCoversTheStoreConfigOnly(t *testing.T) {
	config := Config{Store: Etcd}
	config.Etcd = EtcdConfig{Machines: []string{"1.1.1.1", "2.2.2.2"}, Prefix: "/eru"}

	base, err := config.Identifier()
	assert.NoError(t, err)
	assert.NotEmpty(t, base)

	config.Bind = "5002"
	config.Auth = AuthConfig{Username: "eru", Password: "secret"}
	config.Docker.AuthConfigs = map[string]AuthConfig{"hub": {Password: "secret"}}
	unrelated, err := config.Identifier()
	assert.NoError(t, err)
	assert.Equal(t, base, unrelated)

	config.Etcd.Prefix = "/eru2"
	moved, err := config.Identifier()
	assert.NoError(t, err)
	assert.NotEqual(t, base, moved)
}
