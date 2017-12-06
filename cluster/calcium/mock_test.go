package calcium

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"runtime"
	"strings"
	"sync"

	"github.com/stretchr/testify/mock"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	sourcemock "github.com/projecteru2/core/source/mocks"
	"github.com/projecteru2/core/store/mock"
	coretypes "github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

const (
	podname        = "dev_pod"
	desc           = "dev pod with one node"
	nodename       = "node1"
	updatenodename = "node4"
	image          = "hub.testhub.com/base/alpine:base-2017.03.14"
	APIVersion     = "v1.29"
	mockMemory     = int64(8589934592) // 8G
	mockID         = "f1f9da344e8f8f90f73899ddad02da6cdf2218bbe52413af2bcfef4fba2d22de"
	appmemory      = int64(268435456) // 0.25 G
	driver         = "calico"
)

type Map struct {
	sync.Mutex
	data map[string]container.Resources
}

func (m *Map) Set(id string, res container.Resources) {
	m.Lock()
	defer m.Unlock()
	m.data[id] = res
}

func (m *Map) Get(id string) container.Resources {
	m.Lock()
	defer m.Unlock()
	return m.data[id]
}

var (
	mockCPU = coretypes.CPUMap{"0": 10, "1": 10, "2": 10, "3": 10}
	config  = coretypes.Config{
		Etcd: coretypes.EtcdConfig{
			Machines: []string{""},
		},
		Git: coretypes.GitConfig{SCMType: "gitlab"},
		Docker: coretypes.DockerConfig{
			Hub:       "hub.testhub.com",
			Namespace: "apps",
		},
		Scheduler: coretypes.SchedConfig{
			ShareBase: 10,
			MaxShare:  -1,
		},
	}
	mockc     *calcium
	mockStore *mockstore.MockStore
	err       error
	etpts     = &coretypes.Entrypoint{
		HealthCheck: &coretypes.HealthCheck{
			TCPPorts: []string{"80"},
			HTTPPort: "80",
			HTTPURL:  "x",
			HTTPCode: 200,
		},
	}
	opts = &coretypes.DeployOptions{
		Name:       "appname", // Name of application
		Entrypoint: etpts,     // entrypoint
		Podname:    podname,   // Name of pod to deploy
		Nodename:   nodename,  // Specific nodes to deploy, if given, must belong to pod
		Image:      image,     // Name of image to deploy
		CPUQuota:   1,         // How many cores needed, e.g. 1.5
		Memory:     appmemory, // Memory for container, in bytes
		Count:      5,         // How many containers needed, e.g. 4
		User:       "root",    // User for container
		// NetworkMode string            // Network mode
	}
	ToUpdateContainerIDs = []string{
		"f1f9da344e8f8f90f73899ddad02da6cdf2218bbe52413af2bcfef4fba2d22de",
		"f1f9da344e8f8f90f73899ddad02da6cdf2218bbe52413af2bcfef4fba2d22df",
		"f1f9da344e8f8f90f73899ddad02da6cdf2218bbe52413af2bcfef4fba2d22dg",
	}

	mockInspectedContainerResources = Map{
		data: map[string]container.Resources{},
	}

	mockBuilds = &coretypes.Builds{
		Stages: []string{"test", "step1", "setp2"},
		Builds: map[string]*coretypes.Build{
			"step1": {
				Base:     "alpine:latest",
				Commands: []string{"cp /bin/ls /root/artifact", "date > /root/something"},
				Artifacts: map[string]string{
					"/root/artifact":  "/root/artifact",
					"/root/something": "/root/something",
				},
			},
			"setp2": {
				Base:     "centos:latest",
				Commands: []string{"echo yooo", "sleep 1"},
			},
			"test": {
				Base:     "ubuntu:latest",
				Commands: []string{"date", "echo done"},
			},
		},
	}
)

func testlogF(format interface{}, a ...interface{}) {
	var (
		caller string
		main   string
	)
	_, fn, line, _ := runtime.Caller(1)
	caller = fmt.Sprintf("%s:%d", fn, line)
	s := strings.Split(caller, "/")
	caller = s[len(s)-1]

	switch format.(type) {
	case string:
		main = fmt.Sprintf(format.(string), a...)
	default:
		main = fmt.Sprintf("%v", format)
	}
	fmt.Printf("%s: %s \n", caller, main)
}

func mockContainerID() string {
	return utils.RandomString(64)
}

func mockDockerDoer(r *http.Request) (*http.Response, error) {
	var b []byte
	prefix := fmt.Sprintf("/%s", APIVersion)
	path := strings.TrimPrefix(r.URL.Path, prefix)

	// get container id
	containerID := ""
	if strings.HasPrefix(path, "/containers/") {
		cid := strings.TrimPrefix(path, "/containers/")
		containerID = strings.Split(cid, "/")[0]
	}

	// mock docker responses
	switch path {
	case "/info": // docker info
		testlogF("mock docker info response")
		info := &types.Info{
			ID:         "daemonID",
			Containers: 3,
		}
		b, _ = json.Marshal(info)
	case "/_ping": // just ping
		testlogF("mock docker ping response")
		header := http.Header{}
		header.Add("OSType", "Linux")
		header.Add("API-Version", APIVersion)
		header.Add("Docker-Experimental", "true")
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     header,
		}, nil
	case "/images/create": // docker image pull <>:<>
		query := r.URL.Query()
		fromImage := query.Get("fromImage")
		tag := query.Get("tag")
		testlogF("mock docker create image: %s:%s", fromImage, tag)
		b = []byte("body")
	case "/images/json": // docker images
		testlogF("mock docker list images")
		b, _ = json.Marshal([]types.ImageSummary{
			{
				ID: "image_id",
			},
		})
	case fmt.Sprintf("/images/%s", image): // docker images
		testlogF("mock docker remove image")
		b, _ = json.Marshal([]types.ImageDeleteResponseItem{
			{
				Untagged: image,
			},
			{
				Deleted: image,
			},
		})
	case "/images/image_id": // docker images
		testlogF("mock docker remove image image_id")
		b, _ = json.Marshal([]types.ImageDeleteResponseItem{
			{
				Untagged: "image_id1",
			},
			{
				Deleted: "image_id",
			},
		})
	case "/containers/create":
		name := r.URL.Query().Get("name")
		testlogF("mock create container: %s", name)
		b, err = json.Marshal(container.ContainerCreateCreatedBody{
			ID: mockContainerID(),
		})
	case fmt.Sprintf("/containers/%s/update", containerID):
		testlogF("update container %s", containerID)
		// update container's resources
		updatedConfig := container.UpdateConfig{}
		data, _ := ioutil.ReadAll(r.Body)
		json.Unmarshal(data, &updatedConfig)
		mockInspectedContainerResources.Set(containerID, updatedConfig.Resources)
		b, err = json.Marshal(container.ContainerUpdateOKBody{})
	case fmt.Sprintf("/containers/%s", containerID):
		testlogF("remove container %s", containerID)
		b = []byte("body")
	case fmt.Sprintf("/containers/%s/start", containerID):
		testlogF("start container %s", containerID)
		b = []byte("body")
	case fmt.Sprintf("/containers/%s/stop", containerID):
		testlogF("stop container %s", containerID)
		b = []byte("body")
	case fmt.Sprintf("/containers/%s/archive", containerID): // docker cp
		query := r.URL.Query()
		path := query.Get("path")
		testlogF("mock docker cp to %s", path)
		headercontent, err := json.Marshal(types.ContainerPathStat{
			Name: "name",
			Mode: 0700,
		})
		if err != nil {
			return errorMock(500, err.Error())
		}
		base64PathStat := base64.StdEncoding.EncodeToString(headercontent)
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       ioutil.NopCloser(bytes.NewReader([]byte("content"))),
			Header: http.Header{
				"X-Docker-Container-Path-Stat": []string{base64PathStat},
			},
		}, nil
	case fmt.Sprintf("/containers/%s/json", containerID):
		testlogF("inspect container %s", containerID)
		rscs := container.Resources{
			CPUQuota: utils.CpuPeriodBase,
			Memory:   appmemory,
		}
		nRscs := container.Resources{}
		if mockInspectedContainerResources.Get(containerID).Memory != nRscs.Memory {
			rscs = mockInspectedContainerResources.Get(containerID)
		}
		containerJSON := types.ContainerJSON{
			NetworkSettings: &types.NetworkSettings{},
			ContainerJSONBase: &types.ContainerJSONBase{
				ID:    containerID,
				Image: "image:latest",
				Name:  "name",
				HostConfig: &container.HostConfig{
					Resources: rscs,
				},
			},
			Config: &container.Config{
				Labels: nil,
				Image:  "image:latest",
			},
		}
		mockInspectedContainerResources.Set(containerID, containerJSON.HostConfig.Resources)
		b, _ = json.Marshal(containerJSON)
	case "/networks/bridge/disconnect":
		var disconnect types.NetworkDisconnect
		if err := json.NewDecoder(r.Body).Decode(&disconnect); err != nil {
			return errorMock(500, err.Error())
		}
		testlogF("disconnect container %s from bridge network", disconnect.Container)
		b = []byte("body")
	case "/networks":
		b, _ = json.Marshal([]types.NetworkResource{
			{
				Name:   "mock_network",
				Driver: "bridge",
			},
		})
	case "/build":
		b, _ = json.Marshal([]types.ImageBuildResponse{
			{
				Body:   ioutil.NopCloser(bytes.NewReader([]byte("content"))),
				OSType: "linux",
			},
		})
	}

	if len(b) != 0 {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       ioutil.NopCloser(bytes.NewReader(b)),
		}, nil
	}

	errMsg := fmt.Sprintf("Server Error, unknown path: %s", path)
	return errorMock(500, errMsg)
}

func newMockClient(doer func(*http.Request) (*http.Response, error)) *http.Client {
	r := &http.Client{
		Transport: transportFunc(doer),
	}
	return r
}

func mockDockerClient() *client.Client {
	clnt, err := client.NewClient("http://127.0.0.1", "v1.29", newMockClient(mockDockerDoer), nil)
	if err != nil {
		panic(err)
	}
	return clnt
}

func errorMock(statusCode int, message string) (*http.Response, error) {
	header := http.Header{}
	header.Set("Content-Type", "application/json")

	body, err := json.Marshal(&types.ErrorResponse{
		Message: message,
	})
	if err != nil {
		return nil, err
	}

	return &http.Response{
		StatusCode: statusCode,
		Body:       ioutil.NopCloser(bytes.NewReader(body)),
		Header:     header,
	}, nil
}

// transportFunc allows us to inject a mock transport for testing. We define it
// here so we can detect the tlsconfig and return nil for only this type.
type transportFunc func(*http.Request) (*http.Response, error)

func (tf transportFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return tf(req)
}

func initMockConfig() {
	mockc, err = New(config)
	if err != nil {
		panic(err)
	}

	mockStore = &mockstore.MockStore{}
	mockc.SetStore(mockStore)
	mockSource := &sourcemock.Source{}
	mockc.source = mockSource

	clnt := mockDockerClient()

	n1 := &coretypes.Node{
		Name:      nodename,
		Podname:   podname,
		Endpoint:  "tcp://10.0.0.1:2376",
		CPU:       mockCPU,
		MemCap:    mockMemory,
		Available: true,
		Engine:    clnt,
	}
	n2 := &coretypes.Node{
		Name:      "node2",
		Podname:   podname,
		Endpoint:  "tcp://10.0.0.2:2376",
		CPU:       mockCPU,
		MemCap:    mockMemory,
		Available: true,
		Engine:    clnt,
	}
	n3 := &coretypes.Node{
		Name:      "node3",
		Podname:   podname,
		Endpoint:  "tcp://10.0.0.2:2376",
		CPU:       mockCPU,
		MemCap:    mockMemory,
		Available: true,
		Engine:    clnt,
	}
	n4 := &coretypes.Node{
		Name:      updatenodename,
		Podname:   podname,
		Endpoint:  "tcp://10.0.0.2:2376",
		CPU:       mockCPU,
		MemCap:    mockMemory - appmemory*3,
		Available: true,
		Engine:    clnt,
	}

	pod := &coretypes.Pod{Name: podname, Desc: desc, Favor: "MEM"}
	mockStringType := mock.AnythingOfType("string")
	mockCPUMapType := mock.AnythingOfType("types.CPUMap")
	mockNodeType := mock.AnythingOfType("*types.Node")
	mockStore.On("GetPod", mockStringType).Return(pod, nil)
	mockStore.On("GetNodesByPod", mockStringType).Return([]*coretypes.Node{n1, n2}, nil)
	mockStore.On("GetAllNodes").Return([]*coretypes.Node{n1, n2, n3}, nil)
	mockStore.On("GetNode", podname, "node1").Return(n1, nil)
	mockStore.On("GetNode", podname, "node2").Return(n2, nil)
	mockStore.On("GetNode", podname, "node3").Return(n3, nil)
	mockStore.On("GetNode", podname, updatenodename).Return(n4, nil)
	mockStore.On("GetNode", "", "").Return(n2, nil)

	mockStore.On("UpdateNodeMem", podname, mockStringType, mock.AnythingOfType("int64"), mockStringType).Return(nil)
	mockStore.On("UpdateNodeCPU", podname, mockStringType, mockCPUMapType, mockStringType).Return(nil)
	mockStore.On("UpdateNode", mockNodeType).Return(nil)

	lk := mockstore.MockLock{}
	lk.Mock.On("Lock").Return(nil)
	lk.Mock.On("Unlock").Return(nil)
	mockStore.On("CreateLock", mockStringType, mock.AnythingOfType("int")).Return(&lk, nil)

	mockStore.On("RemoveContainer", mock.MatchedBy(func(input *coretypes.Container) bool {
		return true
	})).Return(nil)

	mockStore.On("RemovePod", mockStringType).Return(nil)

	// 模拟集群上有5个旧容器的情况.
	deployNodeInfo := []coretypes.NodeInfo{
		coretypes.NodeInfo{
			Name:      "node1",
			CPURate:   400000,
			Count:     3,
			Deploy:    0,
			CPUAndMem: coretypes.CPUAndMem{CpuMap: coretypes.CPUMap{"0": 10, "1": 10, "2": 10, "3": 10}, MemCap: 8589934592},
		},
		coretypes.NodeInfo{
			Name:      "node2",
			CPURate:   400000,
			Count:     1,
			Deploy:    0,
			CPUAndMem: coretypes.CPUAndMem{CpuMap: coretypes.CPUMap{"0": 10, "1": 10, "2": 10, "3": 10}, MemCap: 8589934592},
		},
		coretypes.NodeInfo{
			Name:      "node3",
			CPURate:   400000,
			Count:     1,
			Deploy:    0,
			CPUAndMem: coretypes.CPUAndMem{CpuMap: coretypes.CPUMap{"0": 10, "1": 10, "2": 10, "3": 10}, MemCap: 8589934592},
		},
	}

	// make plan
	mockStore.On("MakeDeployStatus", opts, mock.Anything).Return(deployNodeInfo, nil)

	// GetContainer
	rContainer := &coretypes.Container{
		ID:       mockID,
		Engine:   clnt,
		Podname:  podname,
		Nodename: nodename,
		Name:     "hello_hi_123",
	}

	mockStore.On("GetContainer", mockID).Return(rContainer, nil)

	// GetContainers
	rContainers := []*coretypes.Container{}
	for _, mID := range ToUpdateContainerIDs {
		rContainer := &coretypes.Container{
			ID:       mID,
			Engine:   clnt,
			Podname:  podname,
			Nodename: updatenodename,
			Name:     "hello_hi_123",
			CPU:      coretypes.CPUMap{"0": 10},
			Memory:   appmemory,
		}
		rContainers = append(rContainers, rContainer)
	}
	mockStore.On("GetContainers", ToUpdateContainerIDs).Return(rContainers, nil)

	// mockSource
	mockSource.On("SourceCode", mockStringType, mockStringType, mockStringType).Return(nil)
	mockSource.On("Artifact", mockStringType, mockStringType).Return(nil)
	mockSource.On("Security", mockStringType).Return(nil)
}
