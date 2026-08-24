package docker

import (
	"archive/tar"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	dockercontainer "github.com/docker/docker/api/types/container"
	dockernetwork "github.com/docker/docker/api/types/network"
	dockerslice "github.com/docker/docker/api/types/strslice"
	"github.com/docker/go-connections/nat"
	"github.com/docker/go-units"
	"golang.org/x/sync/errgroup"

	"github.com/projecteru2/core/engine"
	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/log"
	resourcetypes "github.com/projecteru2/core/resource/types"
	coretypes "github.com/projecteru2/core/types"
)

const (
	minMemory       = units.MiB * 4
	maxMemory       = math.MaxInt64
	defaultCPUShare = 1024
	root            = "root"
)

// RawArgs carries docker-specific container options through core untouched.
type RawArgs struct {
	PidMode    dockercontainer.PidMode `json:"pid_mod"`
	StorageOpt map[string]string       `json:"storage_opt"`
	CapAdd     []string                `json:"cap_add"`
	CapDrop    []string                `json:"cap_drop"`
	Ulimits    []*units.Ulimit         `json:"ulimits"`
	Runtime    string                  `json:"runtime"`
}

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

func (e *Engine) VirtualizationCreate(ctx context.Context, opts *enginetypes.VirtualizationCreateOptions) (*enginetypes.VirtualizationCreated, error) { //nolint
	logger := log.WithFunc("engine.docker.VirtualizationCreate")
	r := &enginetypes.VirtualizationCreated{}
	var err error

	resourceOpts := &engine.VirtualizationResource{}
	if err = resourceOpts.Decode(opts.EngineParams); err != nil {
		logger.Errorf(ctx, err, "failed to parse engine args %+v", opts.EngineParams)
		return r, coretypes.ErrInvalidEngineArgs
	}

	if resourceOpts.Memory > 0 && resourceOpts.Memory < minMemory || resourceOpts.Memory < 0 {
		return r, coretypes.ErrInvaildMemory
	}
	if opts.Lambda {
		opts.LogType = "json-file"
	}

	restartPolicy := ""
	restartRetry := 0
	restartStr := strings.Split(opts.Restart, ":")
	restartPolicy = restartStr[0]
	if retry, atoiErr := strconv.Atoi(restartStr[len(restartStr)-1]); atoiErr == nil {
		restartRetry = retry
	}
	// networks overrides the configured network mode
	var networkMode dockercontainer.NetworkMode
	networks := map[string]string{}
	for name, network := range opts.Networks {
		networkMode = dockercontainer.NetworkMode(name)
		networks[name] = network
		if networkMode.IsHost() {
			networks[name] = ""
		}
	}
	if networkMode == "" {
		networkMode = dockercontainer.NetworkMode(e.config.Docker.NetworkMode)
	}
	if opts.LogConfig == nil {
		opts.LogConfig = map[string]string{}
	}
	opts.LogConfig["mode"] = "non-blocking"
	opts.LogConfig["max-buffer-size"] = "4m"
	opts.LogConfig["tag"] = fmt.Sprintf("%s {{.ID}}", opts.Name)
	if opts.Debug {
		opts.LogType = e.config.Docker.Log.Type
		maps.Copy(opts.LogConfig, e.config.Docker.Log.Config)
	}
	hostIP := GetIP(ctx, e.client.DaemonHost())
	opts.Env = append(opts.Env, fmt.Sprintf("ERU_NODE_IP=%s", hostIP))
	if len(opts.DNS) == 0 && e.config.Docker.UseLocalDNS && hostIP != "" {
		opts.DNS = []string{hostIP}
	}
	binds, volumes := makeMountPaths(ctx, opts, resourceOpts)
	logger.Debugf(ctx, "app %s will bind %+v", opts.Name, binds)

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
			size, parseErr := strconv.ParseInt(parts[3], 10, 64)
			if parseErr != nil {
				return nil, parseErr
			}
			volumeTotal += size
		}
		if resourceOpts.Storage-volumeTotal > 0 {
			rArgs.StorageOpt["size"] = fmt.Sprintf("%+v", resourceOpts.Storage-volumeTotal)
		}
	}
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
			Name:              dockercontainer.RestartPolicyMode(restartPolicy),
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
			port, portErr := nat.NewPort("tcp", p)
			if portErr != nil {
				return r, portErr
			}
			exposePorts[port] = struct{}{}
			portMapping[port] = []nat.PortBinding{{HostPort: p}}
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

		endpointSetting, settingErr := e.makeIPV4EndpointSetting(ipv4)
		if settingErr != nil {
			return r, settingErr
		}
		ipForShow := ipv4
		if ipForShow == "" {
			ipForShow = "[AutoAlloc]"
		}
		networkConfig.EndpointsConfig[networkID] = endpointSetting
		logger.Infof(ctx, "connect to %s with ip %s", networkID, ipForShow)
	}

	workloadCreated, err := e.client.ContainerCreate(ctx, config, hostConfig, networkConfig, nil, opts.Name)
	r.Name = opts.Name
	r.ID = workloadCreated.ID
	return r, err
}

func (e *Engine) VirtualizationCopyTo(ctx context.Context, ID, target string, content []byte, uid, gid int, mode int64) error {
	return withTarfileDump(ctx, target, content, uid, gid, mode, func(target, tarfile string) error {
		content, err := os.Open(filepath.Clean(tarfile))
		if err != nil {
			return err
		}
		defer func() {
			_ = content.Close()
		}()
		return e.client.CopyToContainer(ctx, ID, filepath.Dir(target), content, dockercontainer.CopyToContainerOptions{AllowOverwriteDirWithFile: true, CopyUIDGID: false})
	})
}

func (e *Engine) VirtualizationCopyChunkTo(ctx context.Context, ID, target string, size int64, content io.Reader, uid, gid int, mode int64) error {
	logger := log.WithFunc("engine.docker.VirtualizationCopyChunkTo")
	pr, pw := io.Pipe()
	tw := tar.NewWriter(pw)
	defer func() {
		_ = tw.Close()
	}()
	g, _ := errgroup.WithContext(ctx)
	g.Go(func() error {
		hdr := &tar.Header{
			Name: filepath.Base(target),
			Size: size,
			Mode: mode,
			Uid:  uid,
			Gid:  gid,
		}
		if taskErr := tw.WriteHeader(hdr); taskErr != nil {
			logger.Errorf(ctx, taskErr, "write header to %s", ID)
			return taskErr
		}
		for {
			data := make([]byte, coretypes.SendLargeFileChunkSize)
			n, taskErr := content.Read(data)
			if taskErr != nil {
				if taskErr != io.EOF {
					logger.Error(ctx, taskErr, "read data from pipe")
					return taskErr
				}
				if closeErr := pw.Close(); closeErr != nil {
					logger.Error(ctx, closeErr, "close pipe writer")
					return closeErr
				}
				return nil
			}
			if n < len(data) {
				data = data[:n]
			}
			_, taskErr = tw.Write(data)
			if taskErr != nil {
				logger.Errorf(ctx, taskErr, "write data into %s", ID)
				if closeErr := pw.Close(); closeErr != nil {
					logger.Error(ctx, closeErr, "close pipe writer")
					return closeErr
				}
				return taskErr
			}
		}
	})
	err := e.client.CopyToContainer(ctx, ID, filepath.Dir(target), pr, dockercontainer.CopyToContainerOptions{AllowOverwriteDirWithFile: true, CopyUIDGID: false})
	if err != nil {
		logger.Errorf(ctx, err, "copy %s to container %s", target, ID)
		return err
	}
	return g.Wait()
}

func (e *Engine) VirtualizationStart(ctx context.Context, ID string) error {
	return e.client.ContainerStart(ctx, ID, dockercontainer.StartOptions{})
}

func (e *Engine) VirtualizationStop(ctx context.Context, ID string, gracefulTimeout time.Duration) error {
	var timeout *int
	if t := int(gracefulTimeout.Seconds()); t > 0 {
		timeout = &t
	}
	return e.client.ContainerStop(ctx, ID, dockercontainer.StopOptions{Timeout: timeout})
}

func (e *Engine) VirtualizationSuspend(context.Context, string) error {
	return nil
}

func (e *Engine) VirtualizationResume(context.Context, string) error {
	return nil
}

func (e *Engine) VirtualizationRemove(ctx context.Context, ID string, removeVolumes, force bool) error {
	if err := e.client.ContainerRemove(ctx, ID, dockercontainer.RemoveOptions{RemoveVolumes: removeVolumes, Force: force}); err != nil {
		if strings.Contains(err.Error(), "no such") {
			err = coretypes.ErrWorkloadNotExists
		}
		return err
	}
	return nil
}

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

func (e *Engine) VirtualizationLogs(ctx context.Context, opts *enginetypes.VirtualizationLogStreamOptions) (stdout, stderr io.ReadCloser, err error) {
	logsOpts := dockercontainer.LogsOptions{
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

func (e *Engine) VirtualizationAttach(ctx context.Context, ID string, stream, stdin bool) (stdout, stderr io.ReadCloser, _ io.WriteCloser, err error) {
	opts := dockercontainer.AttachOptions{
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

func (e *Engine) VirtualizationResize(ctx context.Context, workloadID string, height, width uint) (err error) {
	opts := dockercontainer.ResizeOptions{
		Height: height,
		Width:  width,
	}

	return e.client.ContainerResize(ctx, workloadID, opts)
}

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

func (e *Engine) VirtualizationUpdateResource(ctx context.Context, ID string, engineParams resourcetypes.Resources) error {
	logger := log.WithFunc("engine.docker.VirtualizationUpdateResource")

	resourceOpts := &engine.VirtualizationResource{}
	if err := resourceOpts.Decode(engineParams); err != nil {
		logger.WithField("ID", ID).Errorf(ctx, err, "failed to parse engine args %+v", engineParams)
		return err
	}

	if resourceOpts.Memory > 0 && resourceOpts.Memory < minMemory || resourceOpts.Memory < 0 {
		return coretypes.ErrInvaildMemory
	}
	if len(resourceOpts.Volumes) > 0 || resourceOpts.VolumeChanged {
		logger.Warnf(ctx, "docker engine does not support rebinding volume resource: %+v", resourceOpts.Volumes)
		return coretypes.ErrInvalidVolumeBind
	}

	memory := resourceOpts.Memory
	if memory == 0 {
		memory = maxMemory
	}

	quota := resourceOpts.Quota
	cpuMap := resourceOpts.CPU
	numaNode := resourceOpts.NUMANode
	// docker rejects an empty cpuset, so every cpu is listed explicitly
	if quota == 0 || len(cpuMap) == 0 {
		info, err := e.Info(ctx)
		if err != nil {
			return err
		}
		cpuMap = map[string]int64{}
		for i := range info.NCPU {
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

func (e *Engine) VirtualizationCopyFrom(ctx context.Context, ID, path string) (content []byte, uid, gid int, mode int64, err error) {
	resp, _, err := e.client.CopyFromContainer(ctx, ID, path)
	if err != nil {
		return content, uid, gid, mode, err
	}
	tarReader := tar.NewReader(resp)
	header, err := tarReader.Next()
	if err != nil {
		return content, uid, gid, mode, err
	}
	content, err = io.ReadAll(tarReader)
	return content, header.Uid, header.Gid, header.Mode, err
}

func (e *Engine) RawEngine(context.Context, *enginetypes.RawEngineOptions) (res *enginetypes.RawEngineResult, err error) {
	return nil, nil
}
