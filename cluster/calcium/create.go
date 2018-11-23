package calcium

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	enginetypes "github.com/docker/docker/api/types"
	enginecontainer "github.com/docker/docker/api/types/container"
	enginenetwork "github.com/docker/docker/api/types/network"
	engineslice "github.com/docker/docker/api/types/strslice"
	"github.com/docker/go-connections/nat"
	"github.com/docker/go-units"
	"github.com/projecteru2/core/cluster"
	"github.com/projecteru2/core/metrics"
	"github.com/projecteru2/core/store"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
	"github.com/sanity-io/litter"
	log "github.com/sirupsen/logrus"
)

// CreateContainer use options to create containers
func (c *Calcium) CreateContainer(ctx context.Context, opts *types.DeployOptions) (chan *types.CreateContainerMessage, error) {
	opts.ProcessIdent = utils.RandomString(16)
	pod, err := c.store.GetPod(ctx, opts.Podname)
	if err != nil {
		log.Errorf("[CreateContainer %s] Error during GetPod for %s: %v", opts.ProcessIdent, opts.Podname, err)
		return nil, err
	}
	log.Infof("[CreateContainer %s] Creating container with options:", opts.ProcessIdent)
	litter.Dump(opts)

	// 4194304 Byte = 4 MB, docker 创建容器的内存最低标准
	if opts.Memory < minMemory {
		return nil, types.NewDetailedErr(types.ErrBadMemory,
			fmt.Sprintf("Minimum memory limit allowed is 4MB, got %d", opts.Memory))
	}
	// Count 要大于0
	if opts.Count <= 0 {
		return nil, types.NewDetailedErr(types.ErrBadCount, opts.Count)
	}
	// CPUQuota 也需要大于 0
	if opts.CPUQuota <= 0 {
		return nil, types.NewDetailedErr(types.ErrBadCPU, opts.CPUQuota)
	}

	return c.doCreateContainer(ctx, opts, pod)
}

func (c *Calcium) doCreateContainer(ctx context.Context, opts *types.DeployOptions, pod *types.Pod) (chan *types.CreateContainerMessage, error) {
	ch := make(chan *types.CreateContainerMessage)
	// RFC 计算当前 app 部署情况的时候需要保证同一时间只有这个 app 的这个 entrypoint 在跑
	// 因此需要在这里加个全局锁，直到部署完毕才释放
	// 通过 Processing 状态跟踪达成 18 Oct, 2018
	nodesInfo, err := c.doAllocResource(ctx, opts, pod.Favor)
	if err != nil {
		log.Errorf("[doCreateContainer] Error during alloc resource: %v", err)
		return ch, err
	}

	go func() {
		defer close(ch)
		wg := sync.WaitGroup{}
		wg.Add(len(nodesInfo))
		index := 0

		// do deployment by each node
		for _, nodeInfo := range nodesInfo {
			go metrics.Client.SendDeployCount(nodeInfo.Deploy)
			go func(nodeInfo types.NodeInfo, index int) {
				defer wg.Done()
				defer c.store.DeleteProcessing(ctx, opts, nodeInfo)
				messages := c.doCreateContainerOnNode(ctx, nodeInfo, opts, index)
				for i, m := range messages {
					ch <- m
					if m.Error != nil {
						if m.ContainerID == "" {
							node, nodeLock, err := c.doLockAndGetNode(ctx, opts.Podname, nodeInfo.Name)
							if err != nil {
								log.Errorf("[doCreateContainer] Get and lock node %s failed %v", nodeInfo.Name, err)
								continue
							}
							if err := c.store.UpdateNodeResource(ctx, node, m.CPU, opts.Memory, store.ActionIncr); err != nil {
								log.Errorf("[doCreateContainer] Reset node %s failed %v", nodeInfo.Name, err)
							}
							nodeLock.Unlock(context.Background())
						} else {
							log.Warnf("[doCreateContainer] Container %s not removed", m.ContainerID)
						}
					}
					c.store.UpdateProcessing(ctx, opts, nodeInfo.Name, nodeInfo.Deploy-i-1)
				}
			}(nodeInfo, index)
			index += nodeInfo.Deploy
		}
		wg.Wait()
		// 第一次部署的时候就去cache下镜像吧
		go c.doCacheImage(ctx, opts.Podname, opts.Image)
	}()

	return ch, nil
}

func (c *Calcium) doCreateContainerOnNode(ctx context.Context, nodeInfo types.NodeInfo, opts *types.DeployOptions, index int) []*types.CreateContainerMessage {
	ms := make([]*types.CreateContainerMessage, nodeInfo.Deploy)

	node, err := c.doGetAndPrepareNode(ctx, opts.Podname, nodeInfo.Name, opts.Image)
	if err != nil {
		log.Errorf("[doCreateContainerOnNode] Get and prepare node error %v", err)
		for i := 0; i < nodeInfo.Deploy; i++ {
			cpu := types.CPUMap{}
			if len(nodeInfo.CPUPlan) > 0 {
				cpu = nodeInfo.CPUPlan[i]
			}
			ms[i] = &types.CreateContainerMessage{Error: err, CPU: cpu}
		}
		return ms
	}

	for i := 0; i < nodeInfo.Deploy; i++ {
		// createAndStartContainer will auto cleanup
		cpu := types.CPUMap{}
		if len(nodeInfo.CPUPlan) > 0 {
			cpu = nodeInfo.CPUPlan[i]
		}
		ms[i] = c.doCreateAndStartContainer(ctx, i+index, node, opts, cpu)
		if !ms[i].Success {
			log.Errorf("[doCreateContainerOnNode] Error when create and start a container, %v", ms[i].Error)
			continue
		}
		log.Debugf("[doCreateContainerOnNode] create container success %s", ms[i].ContainerID)
	}

	return ms
}

func (c *Calcium) doGetAndPrepareNode(ctx context.Context, podname, nodename, image string) (*types.Node, error) {
	node, err := c.GetNode(ctx, podname, nodename)
	if err != nil {
		return nil, err
	}

	auth, err := makeEncodedAuthConfigFromRemote(c.config.Docker.AuthConfigs, image)
	if err != nil {
		return nil, err
	}

	return node, pullImage(ctx, node, image, auth)
}

func (c *Calcium) doCreateAndStartContainer(
	ctx context.Context,
	no int, node *types.Node,
	opts *types.DeployOptions,
	cpu types.CPUMap,
) *types.CreateContainerMessage {
	container := &types.Container{
		Podname:    opts.Podname,
		Nodename:   node.Name,
		CPU:        cpu,
		Quota:      opts.CPUQuota,
		Memory:     opts.Memory,
		Hook:       opts.Entrypoint.Hook,
		Privileged: opts.Entrypoint.Privileged,
		Engine:     node.Engine,
		SoftLimit:  opts.SoftLimit,
	}
	createContainerMessage := &types.CreateContainerMessage{
		Podname:  container.Podname,
		Nodename: container.Nodename,
		Success:  false,
		CPU:      cpu,
		Quota:    opts.CPUQuota,
		Memory:   opts.Memory,
		Publish:  map[string][]string{},
	}

	defer func() {
		if !createContainerMessage.Success && container.ID != "" {
			if err := c.doRemoveContainer(ctx, container); err != nil {
				log.Errorf("[doCreateAndStartContainer] create and start container failed, and remove it failed also %v", err)
				return
			}
			createContainerMessage.ContainerID = ""
		}
	}()

	// get config
	config, hostConfig, networkConfig, containerName, err := c.doMakeContainerOptions(no, cpu, opts, node)
	if err != nil {
		createContainerMessage.Error = err
		return createContainerMessage
	}
	container.Name = containerName
	createContainerMessage.ContainerName = container.Name

	// create container
	containerCreated, err := node.Engine.ContainerCreate(ctx, config, hostConfig, networkConfig, containerName)
	if err != nil {
		createContainerMessage.Error = err
		return createContainerMessage
	}
	container.ID = containerCreated.ID
	createContainerMessage.ContainerID = container.ID

	if err = c.store.AddContainer(ctx, container); err != nil {
		createContainerMessage.Error = err
		return createContainerMessage
	}

	// connect container to network
	// if network manager uses docker plugin, then connect must be called before container starts
	// 如果有 networks 的配置，这里的 networkMode 就为 none 了
	if len(opts.Networks) > 0 {
		ctx := utils.ContextWithDockerEngine(ctx, node.Engine)
		// need to ensure all networks are correctly connected
		for networkID, ipv4 := range opts.Networks {
			if err = c.network.ConnectToNetwork(ctx, containerCreated.ID, networkID, ipv4); err != nil {
				createContainerMessage.Error = err
				return createContainerMessage
			}
		}
	}

	// Copy data to container
	if len(opts.Data) > 0 {
		for dst, src := range opts.Data {
			path := filepath.Dir(dst)
			filename := filepath.Base(dst)
			log.Infof("[doCreateAndStartContainer] Copy file %s to dir %s", filename, path)
			log.Debugf("[doCreateAndStartContainer] Local file %s, remote path %s", src, dst)
			f, err := os.Open(src)
			if err != nil {
				createContainerMessage.Error = err
				return createContainerMessage
			}
			if err = node.Engine.CopyToContainer(ctx, containerCreated.ID, path, f, enginetypes.CopyToContainerOptions{AllowOverwriteDirWithFile: true, CopyUIDGID: true}); err != nil {
				createContainerMessage.Error = err
				return createContainerMessage
			}
			f.Close()
		}
	}

	if err = container.Start(ctx); err != nil {
		createContainerMessage.Error = err
		return createContainerMessage
	}

	containerAlived, err := container.Inspect(ctx)
	if err != nil {
		createContainerMessage.Error = err
		return createContainerMessage
	}

	// after start
	if opts.Entrypoint.Hook != nil && len(opts.Entrypoint.Hook.AfterStart) > 0 {
		createContainerMessage.Hook, err = c.doContainerAfterStartHook(
			ctx, container,
			opts.User, opts.Env,
			opts.Entrypoint.Privileged,
		)
		if err != nil {
			createContainerMessage.Error = err
			return createContainerMessage
		}
	}

	// get ips
	if containerAlived.NetworkSettings != nil {
		createContainerMessage.Publish = utils.MakePublishInfo(containerAlived.NetworkSettings.Networks, node.GetIP(), opts.Entrypoint.Publish)
	}

	createContainerMessage.Success = true
	return createContainerMessage
}

func (c *Calcium) doMakeContainerOptions(index int, quota types.CPUMap, opts *types.DeployOptions, node *types.Node) (
	*enginecontainer.Config,
	*enginecontainer.HostConfig,
	*enginenetwork.NetworkingConfig,
	string,
	error) {

	entry := opts.Entrypoint
	// 如果有指定用户，用指定用户
	// 没有指定用户，用镜像自己的
	// CapAdd and Privileged
	user := opts.User
	capAdd := []string{}
	if entry.Privileged {
		user = root
		capAdd = append(capAdd, "SYS_ADMIN")
	}

	// command and user
	// extra args is dynamically
	slices := utils.MakeCommandLineArgs(fmt.Sprintf("%s %s", entry.Command, opts.ExtraArgs))
	cmd := engineslice.StrSlice(slices)

	// env
	nodeIP := node.GetIP()
	env := append(opts.Env, fmt.Sprintf("APP_NAME=%s", opts.Name))
	env = append(env, fmt.Sprintf("ERU_POD=%s", opts.Podname))
	env = append(env, fmt.Sprintf("ERU_NODE_IP=%s", nodeIP))
	env = append(env, fmt.Sprintf("ERU_NODE_NAME=%s", node.Name))
	env = append(env, fmt.Sprintf("ERU_CONTAINER_NO=%d", index))
	env = append(env, fmt.Sprintf("ERU_MEMORY=%d", opts.Memory))

	// mount paths
	binds, volumes := makeMountPaths(opts)
	log.Debugf("[doMakeContainerOptions] App %s will bind %v", opts.Name, binds)

	// log config
	// 默认是配置里的driver, 如果entrypoint有指定就用指定的.
	// 如果用 debug 模式就用默认配置的
	logConfig := c.config.Docker.Log
	if logConfig.Config == nil {
		logConfig.Config = map[string]string{}
	}
	logConfig.Config["tag"] = fmt.Sprintf("%s {{.ID}}", opts.Name)
	if entry.Log != nil && !opts.Debug {
		logConfig.Type = entry.Log.Type
		logConfig.Config = entry.Log.Config
	}

	containerLogConfig := enginecontainer.LogConfig{Type: logConfig.Type, Config: logConfig.Config}

	// basic labels, bind to EruMeta
	containerLabels := map[string]string{
		cluster.ERUMark: "1",
		cluster.ERUMeta: utils.EncodeMetaInLabel(&types.EruMeta{
			Publish:     opts.Entrypoint.Publish,
			HealthCheck: entry.HealthCheck,
		}),
	}

	// 接下来是meta
	for key, value := range opts.Labels {
		containerLabels[key] = value
	}

	// ulimit
	ulimits := []*units.Ulimit{&units.Ulimit{Name: "nofile", Soft: 65535, Hard: 65535}}

	// name
	suffix := utils.RandomString(6)
	containerName := utils.MakeContainerName(opts.Name, opts.Entrypoint.Name, suffix)

	// network mode
	// network mode 和 networks 互斥
	// 没有 networks 的时候用 networkmode 的值
	// 有 networks 的时候一律用 none 作为默认 mode
	networkMode := opts.NetworkMode
	if len(opts.Networks) > 0 {
		for name := range opts.Networks {
			networkMode = name
			break
		}
	} else if networkMode == "" {
		networkMode = c.config.Docker.NetworkMode
	}
	engineNetworkMode := enginecontainer.NetworkMode(networkMode)

	// dns
	// 如果有给dns就优先用给定的dns.
	// 没有给出dns的时候, 如果设定是用宿主机IP作为dns, 就会把宿主机IP设置过去.
	// 其他情况就是默认值.
	// 哦对, networkMode如果是host也不给dns.
	dns := opts.DNS
	if len(dns) == 0 && c.config.Docker.UseLocalDNS && nodeIP != "" && !engineNetworkMode.IsHost() {
		dns = []string{nodeIP}
	}

	// sysctls
	// 只有在非 host 网络下有意义
	sysctls := map[string]string{}
	if !engineNetworkMode.IsHost() {
		sysctls = entry.Sysctls
	}

	config := &enginecontainer.Config{
		Env:             env,
		Cmd:             cmd,
		User:            user,
		Image:           opts.Image,
		Volumes:         volumes,
		WorkingDir:      entry.Dir,
		NetworkDisabled: false,
		Labels:          containerLabels,
		OpenStdin:       opts.OpenStdin,
	}

	resource := makeResourceSetting(opts.CPUQuota, opts.Memory, quota, opts.SoftLimit)
	resource.Ulimits = ulimits

	restartPolicy := entry.RestartPolicy
	maximumRetryCount := 3
	if restartPolicy == restartAlways {
		maximumRetryCount = 0
	}
	hostConfig := &enginecontainer.HostConfig{
		Binds:         binds,
		DNS:           dns,
		LogConfig:     containerLogConfig,
		NetworkMode:   engineNetworkMode,
		RestartPolicy: enginecontainer.RestartPolicy{Name: restartPolicy, MaximumRetryCount: maximumRetryCount},
		CapAdd:        engineslice.StrSlice(capAdd),
		ExtraHosts:    opts.ExtraHosts,
		Privileged:    entry.Privileged,
		Resources:     resource,
		Sysctls:       sysctls,
	}
	networkConfig := &enginenetwork.NetworkingConfig{}

	// Bridge 下 publish 出 Port
	if engineNetworkMode.IsBridge() {
		portMapping := nat.PortMap{}
		exposePorts := nat.PortSet{}
		for _, p := range opts.Entrypoint.Publish {
			port, err := nat.NewPort("tcp", p)
			if err != nil {
				return nil, nil, nil, "", err
			}
			exposePorts[port] = struct{}{}
			portMapping[port] = []nat.PortBinding{}
			portMapping[port] = append(portMapping[port], nat.PortBinding{HostPort: p})
		}
		hostConfig.PortBindings = portMapping
		config.ExposedPorts = exposePorts
	}

	return config, hostConfig, networkConfig, containerName, nil
}
