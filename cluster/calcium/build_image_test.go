package calcium

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"gitlab.ricebook.net/platform/core/network/calico"
	"gitlab.ricebook.net/platform/core/scheduler/simple"
	"gitlab.ricebook.net/platform/core/source/gitlab"
	"gitlab.ricebook.net/platform/core/store/mock"
	"gitlab.ricebook.net/platform/core/types"
)

func TestGetRandomNode(t *testing.T) {
	store := &mockstore.MockStore{}
	config := types.Config{}
	c := &calcium{store: store, config: config, scheduler: simplescheduler.New(), network: calico.New(), source: gitlab.New(config)}

	n1 := &types.Node{Name: "node1", Podname: "podname", Endpoint: "tcp://10.0.0.1:2376", CPU: types.CPUMap{"0": 10, "1": 10}, Available: true}
	n2 := &types.Node{Name: "node2", Podname: "podname", Endpoint: "tcp://10.0.0.2:2376", CPU: types.CPUMap{"0": 10, "1": 10}, Available: true}

	store.On("GetNodesByPod", "podname").Return([]*types.Node{n1, n2}, nil)
	store.On("GetNode", "podname", "node1").Return(n1, nil)
	store.On("GetNode", "podname", "node2").Return(n2, nil)

	node, err := getRandomNode(c, "podname")
	assert.Contains(t, []string{"node1", "node2"}, node.Name)
	assert.Nil(t, err)
}

func TestGetRandomNodeFail(t *testing.T) {
	store := &mockstore.MockStore{}
	config := types.Config{}
	c := &calcium{store: store, config: config, scheduler: simplescheduler.New(), network: calico.New(), source: gitlab.New(config)}

	n1 := &types.Node{Name: "node1", Podname: "podname", Endpoint: "tcp://10.0.0.1:2376", CPU: types.CPUMap{"0": 10, "1": 10}, Available: false}
	n2 := &types.Node{Name: "node2", Podname: "podname", Endpoint: "tcp://10.0.0.2:2376", CPU: types.CPUMap{"0": 10, "1": 10}, Available: false}

	store.On("GetNodesByPod", "podname").Return([]*types.Node{n1, n2}, nil)
	store.On("GetNode", "podname", "node1").Return(n1, nil)
	store.On("GetNode", "podname", "node2").Return(n2, nil)

	node, err := getRandomNode(c, "podname")
	assert.NotNil(t, err)
	assert.Nil(t, node)
}

func TestMultipleBuildDockerFile(t *testing.T) {
	builds := types.ComplexBuild{
		Stages: []string{"test", "step1", "setp2"},
		Builds: map[string]types.Build{
			"step1": types.Build{
				Base:     "alpine:latest",
				Commands: []string{"cp /bin/ls /root/artifact", "date > /root/something"},
				Artifacts: map[string]string{
					"/root/artifact":  "/root/artifact",
					"/root/something": "/root/something",
				},
			},
			"setp2": types.Build{
				Base:     "centos:latest",
				Commands: []string{"echo yooo", "sleep 1"},
			},
			"test": types.Build{
				Base:     "ubuntu:latest",
				Commands: []string{"date", "echo done"},
			},
		},
	}
	appname := "hello-app"
	specs := types.Specs{
		Appname:      appname,
		ComplexBuild: builds,
	}
	rs := richSpecs{specs, "1001", "/app", appname, true}

	tempDIR := os.TempDir()
	err := makeComplexDockerFile(rs, tempDIR)
	assert.NoError(t, err)
	f, _ := os.Open(fmt.Sprintf("%s/Dockerfile", tempDIR))
	bs, _ := ioutil.ReadAll(f)
	fmt.Printf("%s", bs)
	f.Close()
	os.Remove(f.Name())
}

func TestSingleBuildDockerFile(t *testing.T) {
	appname := "hello-app"
	specs := types.Specs{
		Appname: appname,
		Build:   []string{"echo yes", "echo no"},
		Base:    "alpine:latest",
	}
	rs := richSpecs{specs, "1001", "/app", appname, true}

	tempDIR := os.TempDir()
	err := makeSimpleDockerFile(rs, tempDIR)
	assert.NoError(t, err)
	f, _ := os.Open(fmt.Sprintf("%s/Dockerfile", tempDIR))
	bs, _ := ioutil.ReadAll(f)
	fmt.Printf("%s", bs)
	f.Close()
	os.Remove(f.Name())
}

func TestBuildImageError(t *testing.T) {
	initMockConfig()
	reponame := "git@github.com:docker/jira-test.git"
	_, err := mockc.BuildImage(reponame, "x-version", "998", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "No build pod set in config")

	mockc.config.Docker.BuildPod = "dev_pod"
	_, err = mockc.BuildImage(reponame, "x-version", "998", "")
	assert.Contains(t, err.Error(), "Public Key not found")
}

func TestCreateImageTag(t *testing.T) {
	initMockConfig()
	appname := "appname"
	version := "version"
	imageTag := createImageTag(mockc.config, appname, version)
	cfg := mockc.config.Docker
	expectTag := fmt.Sprintf("%s/%s/%s:%s", cfg.Hub, cfg.HubPrefix, appname, version)
	assert.Equal(t, expectTag, imageTag)
}
