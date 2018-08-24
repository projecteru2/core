package calcium

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/projecteru2/core/cluster"
	"github.com/projecteru2/core/types"
	"github.com/stretchr/testify/assert"
)

func TestMakeMountPaths(t *testing.T) {
	opts := &types.DeployOptions{
		Name:         "foo",
		Env:          []string{"A=1"},
		Volumes:      []string{"/foo-data:/foo-data"},
		DeployMethod: cluster.DeployAuto,
	}
	binds, volumes := makeMountPaths(opts)
	assert.Equal(t, binds, []string{"/foo-data:/foo-data:rw"})
	assert.Equal(t, volumes, map[string]struct{}{"/foo-data": struct{}{}})
}

func TestRegistryAuth(t *testing.T) {
	tag := "docker.io/projecteru2/core"
	encodedAuth, _ := makeEncodedAuthConfigFromRemote(mockc.config.Docker.AuthConfigs, tag)
	decodedAuth, _ := base64.StdEncoding.DecodeString(encodedAuth)
	authConfig := types.AuthConfig{}
	json.Unmarshal(decodedAuth, &authConfig)
	assert.Equal(t, authConfig.Password, mockc.config.Docker.AuthConfigs["docker.io"].Password)
}
