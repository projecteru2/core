package fakeengine

import (
	"bufio"
	"bytes"
	"context"
	"io/ioutil"

	"github.com/docker/go-units"
	"github.com/projecteru2/core/engine"
	enginemocks "github.com/projecteru2/core/engine/mocks"
	enginetypes "github.com/projecteru2/core/engine/types"
	coretypes "github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
	mock "github.com/stretchr/testify/mock"
)

const (
	// PrefixKey indicate key prefix
	PrefixKey = "mock://"
)

type writeCloser struct {
	*bufio.Writer
}

// Close close
func (wc *writeCloser) Close() error {
	// Noop
	return nil
}

// MakeClient make a mock client
func MakeClient(ctx context.Context, config coretypes.Config, nodename, endpoint, ca, cert, key string) (engine.API, error) {
	e := &enginemocks.API{}
	// info
	e.On("Info", mock.Anything).Return(&enginetypes.Info{NCPU: 1, MemTotal: units.GiB + 100}, nil)
	// exec
	execID := utils.RandomString(64)
	bw1 := bufio.NewWriter(bytes.NewBuffer([]byte{}))
	writeBuffer1 := &writeCloser{bw1}
	e.On("ExecCreate", mock.Anything, mock.Anything, mock.Anything).Return(execID, nil)
	execData := ioutil.NopCloser(bytes.NewBufferString(execID))
	e.On("ExecAttach", mock.Anything, execID, mock.Anything).Return(execData, writeBuffer1, nil)
	e.On("ExecResize", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	e.On("ExecExitCode", mock.Anything, execID).Return(0, nil)
	// network
	e.On("NetworkConnect", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	e.On("NetworkDisconnect", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	e.On("NetworkList", mock.Anything, mock.Anything).Return([]*enginetypes.Network{&enginetypes.Network{
		Name: "mock-network", Subnets: []string{"1.1.1.1/8", "2.2.2.2/8"},
	}}, nil)
	// image
	e.On("ImageList", mock.Anything, mock.Anything).Return(
		[]*enginetypes.Image{&enginetypes.Image{ID: "mock-image", Tags: []string{"latest"}}}, nil)
	e.On("ImageRemove", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(
		[]string{"mock-image1", "mock-image2"}, nil)
	e.On("ImagesPrune", mock.Anything).Return(nil)
	pullImageData := ioutil.NopCloser(bytes.NewBufferString("pull image layer1 ...\npull image layer2...\n"))
	e.On("ImagePull", mock.Anything, mock.Anything, mock.Anything).Return(pullImageData, nil)
	pushImageData := ioutil.NopCloser(bytes.NewBufferString("{\"stream\":\"push something...\"}\n"))
	e.On("ImagePush", mock.Anything, mock.Anything).Return(pushImageData, nil)
	buildImageData := ioutil.NopCloser(bytes.NewBufferString("{\"stream\":\"build something...\"}\n"))
	e.On("ImageBuild", mock.Anything, mock.Anything, mock.Anything).Return(buildImageData, nil)
	e.On("ImageBuildCachePrune", mock.Anything, mock.Anything).Return(uint64(0), nil)
	imageDigest := utils.RandomString(64)
	e.On("ImageLocalDigests", mock.Anything, mock.Anything).Return([]string{imageDigest}, nil)
	e.On("ImageRemoteDigest", mock.Anything, mock.Anything).Return(imageDigest, nil)
	// build
	e.On("BuildRefs", mock.Anything, mock.Anything, mock.Anything).Return([]string{"ref1", "ref2"})
	buildContent := ioutil.NopCloser(bytes.NewBufferString("this is content"))
	e.On("BuildContent", mock.Anything, mock.Anything, mock.Anything).Return(buildContent, nil)
	// virtualization
	ID := utils.RandomString(64)
	vc := &enginetypes.VirtualizationCreated{ID: ID, Name: "mock-test-cvm"}
	e.On("VirtualizationCreate", mock.Anything, mock.Anything).Return(vc, nil)
	e.On("VirtualizationCopyTo", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	e.On("VirtualizationStart", mock.Anything, mock.Anything).Return(nil)
	e.On("VirtualizationStop", mock.Anything, mock.Anything).Return(nil)
	e.On("VirtualizationRemove", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	vcJSON := &enginetypes.VirtualizationInfo{ID: ID, Image: "mock-image", Running: true, Networks: map[string]string{"mock-network": "1.1.1.1"}}
	e.On("VirtualizationInspect", mock.Anything, mock.Anything).Return(vcJSON, nil)
	logs := ioutil.NopCloser(bytes.NewBufferString("logs1...\nlogs2...\n"))
	e.On("VirtualizationLogs", mock.Anything, mock.Anything).Return(logs, nil)
	attachData := ioutil.NopCloser(bytes.NewBufferString("logs1...\nlogs2...\n"))
	bw := bufio.NewWriter(bytes.NewBuffer([]byte{}))
	writeBuffer := &writeCloser{bw}
	e.On("VirtualizationAttach", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(attachData, writeBuffer, nil)
	e.On("VirtualizationResize", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	e.On("VirtualizationWait", mock.Anything, mock.Anything, mock.Anything).Return(&enginetypes.VirtualizationWaitResult{Message: "", Code: 0}, nil)
	e.On("VirtualizationUpdateResource", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	copyData := ioutil.NopCloser(bytes.NewBufferString("d1...\nd2...\n"))
	e.On("VirtualizationCopyFrom", mock.Anything, mock.Anything, mock.Anything).Return(copyData, "", nil)
	e.On("ResourceValidate", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	return e, nil
}
