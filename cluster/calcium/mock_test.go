package calcium

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/stretchr/testify/mock"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/ioutils"
	"gitlab.ricebook.net/platform/core/store/mock"
	coretypes "gitlab.ricebook.net/platform/core/types"
	"gitlab.ricebook.net/platform/core/utils"
)

const (
	podname    = "dev_pod"
	desc       = "dev pod with one node"
	nodename   = "node1"
	image      = "hub.testhub.com/base/alpine:base-2017.03.14"
	APIVersion = "v1.29"
	mockMemory = int64(8589934592)
	mockID     = "f1f9da344e8f8f90f73899ddad02da6cdf2218bbe52413af2bcfef4fba2d22de"
)

var (
	mockCPU = coretypes.CPUMap{"0": 10, "1": 10, "2": 10, "3": 10}
	config  = coretypes.Config{
		EtcdMachines: []string{""},
		Git:          coretypes.GitConfig{SCMType: "gitlab"},
		Docker: coretypes.DockerConfig{
			Hub:       "hub.testhub.com",
			HubPrefix: "apps",
		},
		Timeout: coretypes.TimeoutConfig{
			Common: 3,
		},
		Scheduler: coretypes.SchedConfig{
			ShareBase: 10,
			MaxShare:  -1,
		},
	}
	mockc            *calcium
	mockStore        *mockstore.MockStore
	err              error
	mockTimeoutError bool
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

func mockReadCloser(ctx context.Context) io.ReadCloser {
	pr, pw := io.Pipe()
	w := ioutils.NewWriteFlusher(pw)
	msgChan := make(chan string)

	go func() {
		for {
			msgChan <- "pulling image...\n"
			time.Sleep(1000 * time.Millisecond)
		}
	}()
	go func() {
		for {
			select {
			case <-ctx.Done():
				testlogF("Error!! Context canceld!!!")
				w.Close()
				pw.Close()
				pr.Close()
				return
			case msg := <-msgChan:
				w.Write([]byte(msg))
			}
		}
	}()

	return pr
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
		if mockTimeoutError {
			ctx := r.Context()
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       mockReadCloser(ctx),
			}, nil
		}
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
		b, _ = json.Marshal(types.ContainerJSON{
			ContainerJSONBase: &types.ContainerJSONBase{
				ID:    containerID,
				Image: "image:latest",
				Name:  "name",
			},
			Config: &container.Config{
				Labels: nil,
				Image:  "image:latest",
			},
		})
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

func mockDockerHTTPClient() *http.Client {
	return newMockClient(mockDockerDoer)
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
	mockc, err = New(utils.SetTimeout(config))
	if err != nil {
		panic(err)
	}

	mockStore = &mockstore.MockStore{}
	mockc.SetStore(mockStore)

	clnt, err := client.NewClient("http://127.0.0.1", "v1.29", mockDockerHTTPClient(), nil)
	if err != nil {
		panic(err)
	}

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

	pod := &coretypes.Pod{Name: podname, Desc: desc, Favor: "MEM"}
	mockStringType := mock.AnythingOfType("string")
	mockStore.On("GetPod", mockStringType).Return(pod, nil)
	mockStore.On("GetNodesByPod", mockStringType).Return([]*coretypes.Node{n1, n2}, nil)
	mockStore.On("GetAllNodes").Return([]*coretypes.Node{n1, n2}, nil)
	mockStore.On("GetNode", podname, "node1").Return(n1, nil)
	mockStore.On("GetNode", podname, "node2").Return(n2, nil)
	mockStore.On("GetNode", "", "").Return(n2, nil)

	mockStore.On("UpdateNodeMem", podname, mockStringType, mock.AnythingOfType("int64"), mockStringType).Return(nil)

	lk := mockstore.MockLock{}
	lk.Mock.On("Lock").Return(nil)
	lk.Mock.On("Unlock").Return(nil)
	mockStore.On("CreateLock", mockStringType, mock.AnythingOfType("int")).Return(&lk, nil)

	mockStore.On("RemoveContainer", mockStringType, mock.MatchedBy(func(input *coretypes.Container) bool {
		return true
	})).Return(nil)

	mockStore.On("DeletePod", mockStringType).Return(nil)

	deployNodeInfo := []coretypes.NodeInfo{
		coretypes.NodeInfo{
			Name:      "node1",
			CPURate:   400000,
			Count:     0,
			Deploy:    2,
			CPUAndMem: coretypes.CPUAndMem{CpuMap: coretypes.CPUMap{"0": 10, "1": 10, "2": 10, "3": 10}, MemCap: 8589934592},
		},
		coretypes.NodeInfo{
			Name:      "node2",
			CPURate:   400000,
			Count:     0,
			Deploy:    1,
			CPUAndMem: coretypes.CPUAndMem{CpuMap: coretypes.CPUMap{"0": 10, "1": 10, "2": 10, "3": 10}, MemCap: 8589934592},
		},
	}
	opts := &coretypes.DeployOptions{
		Appname:    "root",
		Image:      image,
		Podname:    podname,
		Entrypoint: "test",
		Count:      3,
		Memory:     268435456,
		CPUQuota:   1,
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
}
