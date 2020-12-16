package docker

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/docker/go-connections/nat"
	"github.com/docker/go-units"
	"github.com/pkg/errors"

	"github.com/projecteru2/core/log"

	dockertypes "github.com/docker/docker/api/types"
	dockercontainer "github.com/docker/docker/api/types/container"
	dockernetwork "github.com/docker/docker/api/types/network"
	dockerslice "github.com/docker/docker/api/types/strslice"

	"encoding/json"

	enginetypes "github.com/projecteru2/core/engine/types"
	coretypes "github.com/projecteru2/core/types"
)

const (
	minMemory     = units.MiB * 4
	maxMemory     = math.MaxInt64
	restartAlways = "always"
	root          = "root"
)

type rawArgs struct {
	PidMode    dockercontainer.PidMode `json:"pid_mod"`
	StorageOpt map[string]string       `json:"storage_opt"`
	CapAdd     []string                `json:"cap_add"`
	CapDrop    []string                `json:"cap_drop"`
}

// VirtualizationCreate create a workload
func (e *Engine) VirtualizationCreate(ctx context.Context, opts *enginetypes.VirtualizationCreateOptions) (*enginetypes.VirtualizationCreated, error) {
	r := &enginetypes.VirtualizationCreated{}
	// memory should more than 4MiB
	if opts.Memory > 0 && opts.Memory < minMemory || opts.Memory < 0 {
		return r, coretypes.ErrBadMemory
	}
	// set default log driver if lambda
	if opts.Lambda {
		opts.LogType = "json-file"
	}
	// set restart always
	restartRetryCount := 3
	if opts.RestartPolicy == restartAlways {
		restartRetryCount = 0
	}
	// no longer use opts.Network as networkmode
	// always get network name from networks
	// -----------------------------------------
	// network mode 和 networks 互斥
	// 没有 networks 的时候用 networkmode 的值
	// 有 networks 的时候一律用用 networks 的值作为 mode
	var networkMode dockercontainer.NetworkMode
	for name := range opts.Networks {
		networkMode = dockercontainer.NetworkMode(name)
		if networkMode.IsHost() {
			opts.Networks[name] = ""
		}
	}
	// 如果没有 network 用默认值替换
	if networkMode == "" {
		networkMode = dockercontainer.NetworkMode(e.config.Docker.NetworkMode)
	}
	// log config
	if opts.LogConfig == nil {
		opts.LogConfig = map[string]string{}
	}
	opts.LogConfig["mode"] = "non-blocking"
	opts.LogConfig["max-buffer-size"] = "4m"
	opts.LogConfig["tag"] = fmt.Sprintf("%s {{.ID}}", opts.Name)
	if opts.Debug {
		opts.LogType = e.config.Docker.Log.Type
		for k, v := range e.config.Docker.Log.Config {
			opts.LogConfig[k] = v
		}
	}
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
	log.Debugf("[VirtualizationCreate] App %s will bind %v", opts.Name, binds)

	config := &dockercontainer.Config{
		Env:             opts.Env,
		Cmd:             dockerslice.StrSlice(opts.Cmd),
		User:            opts.User,
		Image:           opts.Image,
		Volumes:         volumes,
		WorkingDir:      opts.WorkingDir,
		NetworkDisabled: networkMode == "",
		Labels:          opts.Labels,
		OpenStdin:       opts.Stdin,
		Tty:             opts.Stdin,
	}

	resource := makeResourceSetting(opts.Quota, opts.Memory, opts.CPU, opts.NUMANode)
	// set ulimits
	resource.Ulimits = []*units.Ulimit{
		{Name: "nofile", Soft: 65535, Hard: 65535},
	}
	if networkMode.IsHost() {
		opts.DNS = []string{}
		opts.Sysctl = map[string]string{}
	}
	rArgs := &rawArgs{StorageOpt: map[string]string{}}
	if len(opts.RawArgs) > 0 {
		if err := json.Unmarshal(opts.RawArgs, rArgs); err != nil {
			return r, err
		}
	}
	if opts.Storage > 0 {
		volumeTotal := int64(0)
		for _, v := range opts.Volumes {
			parts := strings.Split(v, ":")
			if len(parts) < 4 {
				continue
			}
			size, err := strconv.ParseInt(parts[3], 10, 64)
			if err != nil {
				return nil, err
			}
			volumeTotal += size
		}
		if opts.Storage-volumeTotal > 0 {
			rArgs.StorageOpt["size"] = fmt.Sprintf("%v", opts.Storage-volumeTotal)
		}
	}
	// 如果有指定用户，用指定用户
	// 没有指定用户，用镜像自己的
	// CapAdd and Privileged
	capAdds := dockerslice.StrSlice(rArgs.CapAdd)
	if opts.Privileged {
		opts.User = root
		capAdds = append(capAdds, "SYS_ADMIN")
	}
	hostConfig := &dockercontainer.HostConfig{
		Binds: binds,
		DNS:   opts.DNS,
		LogConfig: dockercontainer.LogConfig{
			Type:   opts.LogType,
			Config: opts.LogConfig,
		},
		NetworkMode: networkMode,
		RestartPolicy: dockercontainer.RestartPolicy{
			Name:              opts.RestartPolicy,
			MaximumRetryCount: restartRetryCount,
		},
		CapAdd:     capAdds,
		ExtraHosts: opts.Hosts,
		Privileged: opts.Privileged,
		Resources:  resource,
		Sysctls:    opts.Sysctl,
		PidMode:    rArgs.PidMode,
		StorageOpt: rArgs.StorageOpt,
	}

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

	workloadCreated, err := e.client.ContainerCreate(ctx, config, hostConfig, networkConfig, nil, opts.Name)
	r.Name = opts.Name
	r.ID = workloadCreated.ID
	return r, err
}

// VirtualizationCopyTo copy things to virtualization
func (e *Engine) VirtualizationCopyTo(ctx context.Context, ID, target string, content io.Reader, AllowOverwriteDirWithFile, CopyUIDGID bool) error {
	return withTarfileDump(target, content, func(target, tarfile string) error {
		content, err := os.Open(tarfile)
		if err != nil {
			return err
		}
		return e.client.CopyToContainer(ctx, ID, filepath.Dir(target), content, dockertypes.CopyToContainerOptions{AllowOverwriteDirWithFile: AllowOverwriteDirWithFile, CopyUIDGID: CopyUIDGID})
	})
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

	workloadJSON, err := e.client.ContainerInspect(ctx, ID)
	r := &enginetypes.VirtualizationInfo{}
	if err != nil {
		return r, err
	}
	r.ID = workloadJSON.ID
	r.User = workloadJSON.Config.User
	r.Image = workloadJSON.Config.Image
	r.Env = workloadJSON.Config.Env
	r.Labels = workloadJSON.Config.Labels
	r.Running = workloadJSON.State.Running
	r.Networks = map[string]string{}
	for networkName, networkSetting := range workloadJSON.NetworkSettings.Networks {
		ip := networkSetting.IPAddress
		if dockercontainer.NetworkMode(networkName).IsHost() {
			ip = GetIP(e.client.DaemonHost())
		}
		r.Networks[networkName] = ip
	}
	return r, nil
}

// VirtualizationLogs show virtualization logs
func (e *Engine) VirtualizationLogs(ctx context.Context, opts *enginetypes.VirtualizationLogStreamOptions) (io.ReadCloser, error) {
	logsOpts := dockertypes.ContainerLogsOptions{
		ShowStdout: opts.Stdout,
		ShowStderr: opts.Stderr,
		Tail:       opts.Tail,
		Follow:     opts.Follow,
		Since:      opts.Since,
		Until:      opts.Until,
	}
	resp, err := e.client.ContainerLogs(ctx, opts.ID, logsOpts)
	if err != nil {
		return nil, err
	}
	return ioutil.NopCloser(mergeStream(resp)), nil
}

// VirtualizationAttach attach to a virtualization
func (e *Engine) VirtualizationAttach(ctx context.Context, ID string, stream, stdin bool) (io.ReadCloser, io.WriteCloser, error) {
	opts := dockertypes.ContainerAttachOptions{
		Stream: stream,
		Stdin:  stdin,
		Logs:   true,
		Stdout: true,
		Stderr: true,
	}
	resp, err := e.client.ContainerAttach(ctx, ID, opts)
	if err != nil {
		return nil, nil, err
	}
	return ioutil.NopCloser(resp.Reader), resp.Conn, nil
}

// VirtualizationResize resizes remote terminal
func (e *Engine) VirtualizationResize(ctx context.Context, workloadID string, height, width uint) (err error) {
	opts := dockertypes.ResizeOptions{
		Height: height,
		Width:  width,
	}

	return e.client.ContainerResize(ctx, workloadID, opts)
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
	if opts.Memory > 0 && opts.Memory < minMemory || opts.Memory < 0 {
		return coretypes.ErrBadMemory
	}
	if opts.VolumeChanged {
		log.Errorf("[VirtualizationUpdateResource] docker engine not support rebinding volume resource: %v", opts.Volumes)
		return coretypes.ErrNotSupport
	}

	memory := opts.Memory
	if memory == 0 {
		memory = maxMemory
	}

	quota := opts.Quota
	cpuMap := opts.CPU
	numaNode := opts.NUMANode
	// unlimited cpu
	if quota == 0 || len(cpuMap) == 0 {
		info, err := e.Info(ctx) // TODO can fixed in docker engine, support empty Cpusetcpus, or use cache to speed up
		if err != nil {
			return err
		}
		cpuMap = map[string]int64{}
		for i := 0; i < info.NCPU; i++ {
			cpuMap[strconv.Itoa(i)] = int64(e.config.Scheduler.ShareBase)
		}
		if quota == 0 {
			quota = -1
			numaNode = ""
		}
	}

	newResource := makeResourceSetting(quota, memory, cpuMap, numaNode)
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
	tarReader := tar.NewReader(resp)
	_, err = tarReader.Next()
	return ioutil.NopCloser(tarReader), stat.Name, errors.Wrapf(err, "read tarball from docker API failed: %s", path)
}
