package utils

import (
	"bytes"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLoadConfig(t *testing.T) {
	f1 := "test"
	buffer := bytes.NewBufferString(f1)
	fname, err := TempFile(ioutil.NopCloser(buffer))
	assert.NoError(t, err)
	_, err = LoadConfig(fname)
	assert.Error(t, err)
	os.Remove(fname)

	f1 = `log_level: "DEBUG"
bind: ":5001"
statsd: "127.0.0.1:8125"
profile: ":12346"
global_timeout: 300

auth:
    username: admin
    password: password
etcd:
    machines:
        - "http://127.0.0.1:2379"
    lock_prefix: "core/_lock"
git:
    public_key: "***REMOVED***"
    private_key: "***REMOVED***"
    token: "***REMOVED***"
    scm_type: "github"
docker:
    network_mode: "bridge"
    cert_path: "/etc/eru/tls"
    hub: "hub.docker.com"
    namespace: "projecteru2"
    build_pod: "eru-test"
    local_dns: true
`

	buffer = bytes.NewBufferString(f1)
	fname, err = TempFile(ioutil.NopCloser(buffer))
	assert.NoError(t, err)
	config, err := LoadConfig(fname)
	assert.NoError(t, err)
	assert.Equal(t, config.LockTimeout, defaultTTL)
	assert.Equal(t, config.GlobalTimeout, time.Duration(time.Second*300))
	assert.Equal(t, config.Etcd.Prefix, defaultPrefix)
	assert.Equal(t, config.Docker.Log.Type, "journald")
	assert.Equal(t, config.Docker.APIVersion, "1.32")
	assert.Equal(t, config.Scheduler.MaxShare, -1)
	assert.Equal(t, config.Scheduler.ShareBase, 100)
	os.Remove(fname)
}
