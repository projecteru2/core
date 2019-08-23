package docker

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"

	"github.com/docker/go-connections/nat"
	"github.com/docker/go-units"

	log "github.com/sirupsen/logrus"

	dockertypes "github.com/docker/docker/api/types"
	dockercontainer "github.com/docker/docker/api/types/container"
	dockernetwork "github.com/docker/docker/api/types/network"
	dockerslice "github.com/docker/docker/api/types/strslice"

	"encoding/json"

	enginetypes "github.com/projecteru2/core/engine/types"
	coretypes "github.com/projecteru2/core/types"
)

type rawArgs struct {
	PidMode dockercontainer.PidMode `json:"pid_mod"`
}

// VirtualizationCreate create a container
func (e *Engine) VirtualizationCreate(ctx context.Context, opts *enginetypes.VirtualizationCreateOptions) (*enginetypes.VirtualizationCreated, error) {
	r := &enginetypes.VirtualizationCreated{}
	// add node IP
	hostIP := GetIP(e.client.DaemonHost())
	opts.Env = append(opts.Env, fmt.Sprintf("ERU_NODE_IP=%s", hostIP))
	// 如果有给dns就优先用给定的dns.
	// 没有给出dns的时候, 如果设定是用宿主机IP作为dns, 就会把宿主机IP设置过去.
	// 其他情况就是默认值.
	// 哦对, networkMode如果是host也不给dns.
	if len(opts.DNS) == 0 && e.config.Docker.UseLocalDNS && hostIP != "" {
		opts.DNS = []string{hostIP}
	}

	// mount paths
	binds, volumes := makeMountPaths(opts)
	log.Debugf("[doMakeContainerOptions] App %s will bind %v", opts.Name, binds)

	config := &dockercontainer.Config{
		Env:             opts.Env,
		Cmd:             dockerslice.StrSlice(opts.Cmd),
		User:            opts.User,
		Image:           opts.Image,
		Volumes:         volumes,
		WorkingDir:      opts.WorkingDir,
		NetworkDisabled: opts.NetworkDisabled,
		Labels:          opts.Labels,
		OpenStdin:       opts.Stdin,
	}

	resource := makeResourceSetting(opts.Quota, opts.Memory, opts.CPU, opts.NUMANode, opts.SoftLimit)

	resource.Ulimits = []*units.Ulimit{}
	for name, u := range opts.Ulimits {
		ulimits := &units.Ulimit{Name: name, Soft: u.Soft, Hard: u.Hard}
		resource.Ulimits = append(resource.Ulimits, ulimits)
	}
	if dockercontainer.NetworkMode(opts.Network).IsHost() {
		opts.DNS = []string{}
		opts.Sysctl = map[string]string{}
		// fix issue #78
		if _, ok := opts.Networks[opts.Network]; ok {
			opts.Networks[opts.Network] = ""
		}
	}

	hostConfig := &dockercontainer.HostConfig{
		Binds: binds,
		DNS:   opts.DNS,
		LogConfig: dockercontainer.LogConfig{
			Type:   opts.LogType,
			Config: opts.LogConfig,
		},
		NetworkMode: dockercontainer.NetworkMode(opts.Network),
		RestartPolicy: dockercontainer.RestartPolicy{
			Name:              opts.RestartPolicy,
			MaximumRetryCount: opts.RestartRetryCount,
		},
		CapAdd:     dockerslice.StrSlice(opts.CapAdd),
		ExtraHosts: opts.Hosts,
		Privileged: opts.Privileged,
		Resources:  resource,
		Sysctls:    opts.Sysctl,
	}

	rArgs := &rawArgs{}
	if err := json.Unmarshal(opts.RawArgs, rArgs); err != nil {
		return r, err
	}
	hostConfig.PidMode = rArgs.PidMode

	if hostConfig.NetworkMode.IsBridge() {
		portMapping := nat.PortMap{}
		exposePorts := nat.PortSet{}
		for _, p := range opts.Publish {
			port, err := nat.NewPort("tcp", p)
			if err != nil {
				return r, err
			}
			exposePorts[port] = struct{}{}
			portMapping[port] = []nat.PortBinding{}
			portMapping[port] = append(portMapping[port], nat.PortBinding{HostPort: p})
		}
		hostConfig.PortBindings = portMapping
		config.ExposedPorts = exposePorts
	}

	networkConfig := &dockernetwork.NetworkingConfig{
		EndpointsConfig: map[string]*dockernetwork.EndpointSettings{},
	}
	for networkID, ipv4 := range opts.Networks {
		endpointSetting, err := e.makeIPV4EndpointSetting(ipv4)
		if err != nil {
			return r, err
		}
		ipForShow := ipv4
		if ipForShow == "" {
			ipForShow = "[AutoAlloc]"
		}
		networkConfig.EndpointsConfig[networkID] = endpointSetting
		log.Infof("[ConnectToNetwork] Connect to %v with IP %v", networkID, ipForShow)
	}

	containerCreated, err := e.client.ContainerCreate(ctx, config, hostConfig, networkConfig, opts.Name)
	r.Name = opts.Name
	r.ID = containerCreated.ID
	return r, err
}

// VirtualizationCopyTo copy things to virtualization
func (e *Engine) VirtualizationCopyTo(ctx context.Context, ID, path string, content io.Reader, AllowOverwriteDirWithFile, CopyUIDGID bool) error {
	return e.client.CopyToContainer(ctx, ID, path, content, dockertypes.CopyToContainerOptions{AllowOverwriteDirWithFile: AllowOverwriteDirWithFile, CopyUIDGID: CopyUIDGID})
}

// VirtualizationStart start virtualization
func (e *Engine) VirtualizationStart(ctx context.Context, ID string) error {
	return e.client.ContainerStart(ctx, ID, dockertypes.ContainerStartOptions{})
}

// VirtualizationStop stop virtualization
func (e *Engine) VirtualizationStop(ctx context.Context, ID string) error {
	return e.client.ContainerStop(ctx, ID, nil)
}

// VirtualizationRemove remove virtualization
func (e *Engine) VirtualizationRemove(ctx context.Context, ID string, removeVolumes, force bool) error {
	return e.client.ContainerRemove(ctx, ID, dockertypes.ContainerRemoveOptions{RemoveVolumes: removeVolumes, Force: force})
}

// VirtualizationInspect get virtualization info
func (e *Engine) VirtualizationInspect(ctx context.Context, ID string) (*enginetypes.VirtualizationInfo, error) {
	if e.client == nil {
		return nil, coretypes.ErrNilEngine
	}

	containerJSON, err := e.client.ContainerInspect(ctx, ID)
	r := &enginetypes.VirtualizationInfo{}
	if err != nil {
		return r, err
	}
	r.ID = containerJSON.ID
	r.User = containerJSON.Config.User
	r.Image = containerJSON.Config.Image
	r.Env = containerJSON.Config.Env
	r.Labels = containerJSON.Config.Labels
	r.Running = containerJSON.State.Running
	r.Networks = map[string]string{}
	for networkName, networkSetting := range containerJSON.NetworkSettings.Networks {
		ip := networkSetting.IPAddress
		if dockercontainer.NetworkMode(networkName).IsHost() {
			ip = GetIP(e.client.DaemonHost())
		}
		r.Networks[networkName] = ip
	}
	return r, nil
}

// VirtualizationLogs show virtualization logs
func (e *Engine) VirtualizationLogs(ctx context.Context, ID string, follow, stdout, stderr bool) (io.Reader, error) {
	logsOpts := dockertypes.ContainerLogsOptions{Follow: follow, ShowStdout: stdout, ShowStderr: stderr}
	resp, err := e.client.ContainerLogs(ctx, ID, logsOpts)
	if err != nil {
		return nil, err
	}
	return mergeStream(ioutil.NopCloser(resp)), nil
}

// VirtualizationAttach attach to a virtualization
func (e *Engine) VirtualizationAttach(ctx context.Context, ID string, stream, stdin bool) (io.ReadCloser, io.WriteCloser, error) {
	resp, err := e.client.ContainerAttach(ctx, ID, dockertypes.ContainerAttachOptions{Stream: stream, Stdin: stdin})
	if err != nil {
		return nil, nil, err
	}
	return ioutil.NopCloser(resp.Reader), resp.Conn, nil
}

// VirtualizationWait wait virtualization exit
func (e *Engine) VirtualizationWait(ctx context.Context, ID, state string) (*enginetypes.VirtualizationWaitResult, error) {
	waitBody, errorCh := e.client.ContainerWait(ctx, ID, dockercontainer.WaitConditionNotRunning)
	r := &enginetypes.VirtualizationWaitResult{}
	select {
	case b := <-waitBody:
		if b.Error != nil {
			r.Message = b.Error.Message
		}
		r.Code = b.StatusCode
		return r, nil
	case err := <-errorCh:
		r.Message = err.Error()
		r.Code = -1
		return r, err
	}
}

// VirtualizationUpdateResource update virtualization resource
func (e *Engine) VirtualizationUpdateResource(ctx context.Context, ID string, opts *enginetypes.VirtualizationResource) error {
	newResource := makeResourceSetting(opts.Quota, opts.Memory, opts.CPU, opts.NUMANode, opts.SoftLimit)
	updateConfig := dockercontainer.UpdateConfig{Resources: newResource}
	_, err := e.client.ContainerUpdate(ctx, ID, updateConfig)
	return err
}

// VirtualizationCopyFrom copy thing from a virtualization
func (e *Engine) VirtualizationCopyFrom(ctx context.Context, ID, path string) (io.ReadCloser, string, error) {
	resp, stat, err := e.client.CopyFromContainer(ctx, ID, path)
	if err != nil {
		return nil, "", err
	}
	return resp, stat.Name, err
}
