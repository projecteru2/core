package etcdv3

import (
	"testing"
	"time"

	"github.com/projecteru2/core/types"

	"github.com/stretchr/testify/assert"
)

func NewMercury(t *testing.T) *Mercury {
	config := types.Config{}
	config.LockTimeout = 10 * time.Second
	config.GlobalTimeout = 30 * time.Second
	config.Etcd = types.EtcdConfig{
		Machines:   []string{"127.0.0.1:2379"},
		Prefix:     "/eru-test",
		LockPrefix: "/eru-test-lock",
	}
	config.MaxConcurrency = 20
	//	config.Docker.CertPath = "/tmp"

	m, err := New(config, t)
	assert.NoError(t, err)
	return m
}
