package calcium

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/projecteru2/core/types"
	"github.com/stretchr/testify/assert"
)

func TestBuildDockerFile(t *testing.T) {
	initMockConfig()

	appname := "hello-app"
	opts := &types.BuildOptions{
		Name:   appname,
		User:   "username",
		UID:    999,
		Tag:    "tag",
		Builds: mockBuilds,
	}

	tempDIR := os.TempDir()
	err := mockc.makeDockerFile(opts, tempDIR)
	assert.NoError(t, err)
	f, _ := os.Open(fmt.Sprintf("%s/Dockerfile", tempDIR))
	bs, _ := ioutil.ReadAll(f)
	fmt.Printf("%s", bs)
	f.Close()
	os.Remove(f.Name())
}

func TestBuildImageError(t *testing.T) {
	initMockConfig()
	ctx := context.Background()
	opts := &types.BuildOptions{
		Name:   "appname",
		User:   "username",
		UID:    998,
		Tag:    "tag",
		Builds: mockBuilds,
	}
	_, err := mockc.BuildImage(ctx, opts)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "No build pod set in config")

	mockc.config.Docker.BuildPod = "dev_pod"
	_, err = mockc.BuildImage(ctx, opts)
	assert.NoError(t, err)
}

func TestCreateImageTag(t *testing.T) {
	initMockConfig()
	appname := "appname"
	version := "version"
	imageTag := createImageTag(mockc.config.Docker, appname, version)
	cfg := mockc.config.Docker
	prefix := strings.Trim(cfg.Namespace, "/")
	expectTag := fmt.Sprintf("%s/%s/%s:%s", cfg.Hub, prefix, appname, version)
	assert.Equal(t, expectTag, imageTag)
}
