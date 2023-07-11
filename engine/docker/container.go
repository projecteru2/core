package docker

import (
	"archive/tar"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	dockertypes "github.com/docker/docker/api/types"
	dockercontainer "github.com/docker/docker/api/types/container"
	dockernetwork "github.com/docker/docker/api/types/network"
	dockerslice "github.com/docker/docker/api/types/strslice"
	"github.com/docker/go-connections/nat"
	"github.com/docker/go-units"
	"github.com/mitchellh/mapstructure"

	"github.com/projecteru2/core/engine"
	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/log"
	resourcetypes "github.com/projecteru2/core/resource/types"
	"github.com/projecteru2/core/types"
	coretypes "github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

const (
	minMemory       = units.MiB * 4
	maxMemory       = math.MaxInt64
	defaultCPUShare = 1024
	root            = "root"
)

// RawArgs means some underlay args
type RawArgs struct {
	PidMode    dockercontainer.PidMode `json:"pid_mod"`
	StorageOpt map[string]string       `json:"storage_opt"`
	CapAdd     []string                `json:"cap_add"`
	CapDrop    []string                `json:"cap_drop"`
	Ulimits    []*units.Ulimit         `json:"ulimits"`
	Runtime    string                  `json:"runtime"`
}

// ensureValues checks if value is nil,
// if so, initiate the value.
// Though a nil slice won't panic in this situation,
// still we initiate the values.
func (r *RawArgs) ensureValues() {
	if r.StorageOpt == nil {
		r.StorageOpt = map[string]string{}
	}
	if r.CapAdd == nil {
		r.CapAdd = []string{}
	}
	if r.CapDrop == nil {
		r.CapDrop = []string{}
	}
	if r.Ulimits == nil {
		r.Ulimits = []*units.Ulimit{}
	}
}

// loadRawArgs loads RawArgs, if b is given,
// values from b will over write default values.
func loadRawArgs(b []byte) (*RawArgs, error) {
	r := &RawArgs{}
	if len(b) > 0 {
		if err := json.Unmarshal(b, r); err != nil {
			return nil, err
		}
	}
	r.ensureValues()
	return r, nil
}

// VirtualizationCreate create a workload
func (e *Engine) VirtualizationCreate(ctx context.Context, opts *enginetypes.VirtualizationCreateOptions) (*enginetypes.VirtualizationCreated, error) { //nolint
	logger := log.WithFunc("engine.docker.VirtualizationCreate")
	r := &enginetypes.VirtualizationCreated{}
	var err error

	// parse engine args to resource options
	resourceOpts := &engine.VirtualizationResource{}
	if err = engine.MakeVirtualizationResource(opts.EngineParams, resourceOpts, func(p resourcetypes.Resources, d *engine.VirtualizationResource) error {
		for _, v := range p {
			if err := mapstructure.Decode(v, d); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		logger.Errorf(ctx, err, "failed to parse engine args %+v", opts.EngineParams)
		return r, coretypes.ErrInvalidEngineArgs
	}

	// memory should more than 4MiB
	if resourceOpts.Memory > 0 && resourceOpts.Memory < minMemory || resourceOpts.Memory < 0 {
		return r, coretypes.ErrInvaildMemory
	}
	// set default log driver if lambda
	if opts.Lambda {
		opts.LogType = "json-file"
	}

	restartPolicy := ""
	restartRetry := 0
	restartStr := strings.Split(opts.Restart, ":")
	restartPolicy = restartStr[0]
	if r, err := strconv.Atoi(restartStr[len(restartStr)-1]); err == nil {
		restartRetry = r
	}
	// no longer use opts.Network as networkmode
	// always get network name from networks
	// -----------------------------------------
	// network mode 和 networks 互斥
	// 没有 networks 的时候用 networkmode 的值
	// 有 networks 的时候一律用用 networks 的值作为 mode
	var networkMode dockercontainer.NetworkMode
	networks := map[string]string{}
	for name, network := range opts.Networks {
		networkMode = dockercontainer.NetworkMode(name)
		networks[name] = network
		if networkMode.IsHost() {
			networks[name] = ""
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
	hostIP := GetIP(ctx, e.client.DaemonHost())
	opts.Env = append(opts.Env, fmt.Sprintf("ERU_NODE_IP=%s", hostIP))
	// 如果有给dns就优先用给定的dns.
	// 没有给出dns的时候, 如果设定是用宿主机IP作为dns, 就会把宿主机IP设置过去.
	// 其他情况就是默认值.
	// 哦对, networkMode如果是host也不给dns.
	if len(opts.DNS) == 0 && e.config.Docker.UseLocalDNS && hostIP != "" {
		opts.DNS = []string{hostIP}
	}
	// mount paths
	binds, volumes := makeMountPaths(ctx, opts, resourceOpts)
	logger.Debugf(ctx, "App %s will bind %+v", opts.Name, binds)

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

	rArgs, err := loadRawArgs(opts.RawArgs)
	if err != nil {
		return r, err
	}

	resource := makeResourceSetting(resourceOpts.Quota, resourceOpts.Memory, resourceOpts.CPU, resourceOpts.NUMANode, resourceOpts.IOPSOptions, false)
	// set ulimits
	if len(rArgs.Ulimits) == 0 {
		resource.Ulimits = []*units.Ulimit{
			{Name: "nofile", Soft: 65535, Hard: 65535},
		}
	} else {
		resource.Ulimits = rArgs.Ulimits
	}
	if networkMode.IsHost() {
		opts.DNS = []string{}
		opts.Sysctl = map[string]string{}
	}
	if resourceOpts.Storage > 0 {
		volumeTotal := int64(0)
		for _, v := range resourceOpts.Volumes {
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
		if resourceOpts.Storage-volumeTotal > 0 {
			rArgs.StorageOpt["size"] = fmt.Sprintf("%+v", resourceOpts.Storage-volumeTotal)
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
			Name:              restartPolicy,
			MaximumRetryCount: restartRetry,
		},
		CapAdd:     capAdds,
		ExtraHosts: opts.Hosts,
		Privileged: opts.Privileged,
		Resources:  resource,
		Sysctls:    opts.Sysctl,
		PidMode:    rArgs.PidMode,
		StorageOpt: rArgs.StorageOpt,
		Runtime:    rArgs.Runtime,
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
	for networkID, ipv4 := range networks {
		if useCNI(opts.Labels) && ipv4 != "" {
			config.Labels["ipv4"] = ipv4
			break
		}

		endpointSetting, err := e.makeIPV4EndpointSetting(ipv4)
		if err != nil {
			return r, err
		}
		ipForShow := ipv4
		if ipForShow == "" {
			ipForShow = "[AutoAlloc]"
		}
		networkConfig.EndpointsConfig[networkID] = endpointSetting
		logger.Infof(ctx, "Connect to %+v with IP %+v", networkID, ipForShow)
	}

	workloadCreated, err := e.client.ContainerCreate(ctx, config, hostConfig, networkConfig, nil, opts.Name)
	r.Name = opts.Name
	r.ID = workloadCreated.ID
	return r, err
}

// VirtualizationCopyTo copy things to virtualization
func (e *Engine) VirtualizationCopyTo(ctx context.Context, ID, target string, content []byte, uid, gid int, mode int64) error {
	return withTarfileDump(ctx, target, content, uid, gid, mode, func(target, tarfile string) error {
		content, err := os.Open(tarfile)
		if err != nil {
			return err
		}
		defer content.Close()
		return e.client.CopyToContainer(ctx, ID, filepath.Dir(target), content, dockertypes.CopyToContainerOptions{AllowOverwriteDirWithFile: true, CopyUIDGID: false})
	})
}

// VirtualizationCopyChunkTo copy chunk to virtualization
func (e *Engine) VirtualizationCopyChunkTo(ctx context.Context, ID, target string, size int64, content io.Reader, uid, gid int, mode int64) error {
	pr, pw := io.Pipe()
	tw := tar.NewWriter(pw)
	defer tw.Close()
	const maxChunkSize = 2 << 10
	// todo 这里有点奇怪，之前带参数的匿名函数会随机报错，现在改成无参的函数后就不报错了，还没找到原因, 且之前的参数writer用的是interface
	utils.SentryGo(func() {
		hdr := &tar.Header{
			Name: filepath.Base(target),
			Size: size,
			Mode: mode,
			Uid:  uid,
			Gid:  gid,
		}
		if err := tw.WriteHeader(hdr); err != nil {
			log.Errorf(ctx, err, "[VirtualizationCopyChunkTo] write header to %s err, err: %v", ID, err)
			return
		}
		for {
			data := make([]byte, maxChunkSize)
			n, err := content.Read(data)
			if err != nil {
				if err != io.EOF {
					log.Errorf(ctx, err, "[VirtualizationCopyChunkTo] read data from pipe err, err: %v", err)
				}
				err := pw.Close()
				if err != nil {
					log.Errorf(ctx, err, "[VirtualizationCopyChunkTo] close pipe writer, err: %v", err)
				}
				return
			}
			if n < len(data) {
				data = data[:n]
			}
			_, err = tw.Write(data)
			if err != nil {
				log.Debugf(ctx, "[VirtualizationCopyChunkTo] write data into %s err, err: %v", ID, err)
				err := pw.Close()
				if err != nil {
					log.Errorf(ctx, err, "[VirtualizationCopyChunkTo] close pipe writer, err: %v", err)
				}
				return
			}
		}
	})
	err := e.client.CopyToContainer(ctx, ID, filepath.Dir(target), pr, dockertypes.CopyToContainerOptions{AllowOverwriteDirWithFile: true, CopyUIDGID: false})
	if err != nil {
		log.Errorf(ctx, err, "[VirtualizationCopyChunkTo] copy %s to container %s err, err:%v", target, ID, err)
	}
	return err
}

// VirtualizationStart start virtualization
func (e *Engine) VirtualizationStart(ctx context.Context, ID string) error {
	return e.client.ContainerStart(ctx, ID, dockertypes.ContainerStartOptions{})
}

// VirtualizationStop stop virtualization
func (e *Engine) VirtualizationStop(ctx context.Context, ID string, gracefulTimeout time.Duration) error {
	var timeout *int
	if t := int(gracefulTimeout.Seconds()); t > 0 {
		timeout = &t
	}
	return e.client.ContainerStop(ctx, ID, dockercontainer.StopOptions{Timeout: timeout})
}

// VirtualizationSuspend suspends virtualization
func (e *Engine) VirtualizationSuspend(context.Context, string) error {
	return nil
}

// VirtualizationResume resumes virtualization
func (e *Engine) VirtualizationResume(context.Context, string) error {
	return nil
}

// VirtualizationRemove remove virtualization
func (e *Engine) VirtualizationRemove(ctx context.Context, ID string, removeVolumes, force bool) error {
	if err := e.client.ContainerRemove(ctx, ID, dockertypes.ContainerRemoveOptions{RemoveVolumes: removeVolumes, Force: force}); err != nil {
		if strings.Contains(err.Error(), "no such") {
			err = types.ErrWorkloadNotExists
		}
		return err
	}
	return nil
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
			ip = GetIP(ctx, e.client.DaemonHost())
		}
		r.Networks[networkName] = ip
	}
	return r, nil
}

// VirtualizationLogs show virtualization logs
func (e *Engine) VirtualizationLogs(ctx context.Context, opts *enginetypes.VirtualizationLogStreamOptions) (stdout, stderr io.ReadCloser, err error) {
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
		return nil, nil, err
	}
	if !opts.Stderr {
		return io.NopCloser(mergeStream(resp)), nil, nil
	}
	stdout, stderr = e.demultiplexStdStream(ctx, resp)
	return stdout, stderr, nil
}

// VirtualizationAttach attach to a virtualization
func (e *Engine) VirtualizationAttach(ctx context.Context, ID string, stream, stdin bool) (stdout, stderr io.ReadCloser, _ io.WriteCloser, err error) {
	opts := dockertypes.ContainerAttachOptions{
		Stream: stream,
		Stdin:  stdin,
		Logs:   true,
		Stdout: true,
		Stderr: true,
	}
	resp, err := e.client.ContainerAttach(ctx, ID, opts)
	if err != nil {
		return nil, nil, nil, err
	}
	if stdin {
		return io.NopCloser(resp.Reader), nil, resp.Conn, nil
	}
	stdout, stderr = e.demultiplexStdStream(ctx, resp.Reader)
	return stdout, stderr, resp.Conn, nil
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
func (e *Engine) VirtualizationWait(ctx context.Context, ID, _ string) (*enginetypes.VirtualizationWaitResult, error) {
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
func (e *Engine) VirtualizationUpdateResource(ctx context.Context, ID string, engineParams resourcetypes.Resources) error {
	logger := log.WithFunc("engine.docker.VirtualizationUpdateResource")

	// parse engine args to resource options
	resourceOpts := &engine.VirtualizationResource{}
	if err := engine.MakeVirtualizationResource(engineParams, resourceOpts, func(p resourcetypes.Resources, d *engine.VirtualizationResource) error {
		for _, v := range p {
			if err := mapstructure.Decode(v, d); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		logger.WithField("ID", ID).Errorf(ctx, err, "failed to parse engine args %+v", engineParams)
		return err
	}

	if resourceOpts.Memory > 0 && resourceOpts.Memory < minMemory || resourceOpts.Memory < 0 {
		return coretypes.ErrInvaildMemory
	}
	if len(resourceOpts.Volumes) > 0 || resourceOpts.VolumeChanged {
		logger.Warnf(ctx, "docker engine not support rebinding volume resource: %+v", resourceOpts.Volumes)
		return coretypes.ErrInvalidVolumeBind
	}

	memory := resourceOpts.Memory
	if memory == 0 {
		memory = maxMemory
	}

	quota := resourceOpts.Quota
	cpuMap := resourceOpts.CPU
	numaNode := resourceOpts.NUMANode
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

	newResource := makeResourceSetting(quota, memory, cpuMap, numaNode, resourceOpts.IOPSOptions, resourceOpts.Remap)
	updateConfig := dockercontainer.UpdateConfig{Resources: newResource}
	_, err := e.client.ContainerUpdate(ctx, ID, updateConfig)
	return err
}

// VirtualizationCopyFrom copy thing from a virtualization
func (e *Engine) VirtualizationCopyFrom(ctx context.Context, ID, path string) (content []byte, uid, gid int, mode int64, err error) {
	resp, _, err := e.client.CopyFromContainer(ctx, ID, path)
	if err != nil {
		return
	}
	tarReader := tar.NewReader(resp)
	header, err := tarReader.Next()
	if err != nil {
		return
	}
	content, err = io.ReadAll(tarReader)
	return content, header.Uid, header.Gid, header.Mode, err
}
