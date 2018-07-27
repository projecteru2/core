package calcium

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	enginetypes "github.com/docker/docker/api/types"
	enginecontainer "github.com/docker/docker/api/types/container"
	enginenetwork "github.com/docker/docker/api/types/network"
	engineslice "github.com/docker/docker/api/types/strslice"
	"github.com/docker/go-units"
	"github.com/projecteru2/core/scheduler"
	"github.com/projecteru2/core/stats"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
	log "github.com/sirupsen/logrus"
)

// CreateContainer use options to create containers
func (c *Calcium) CreateContainer(ctx context.Context, opts *types.DeployOptions) (chan *types.CreateContainerMessage, error) {
	pod, err := c.store.GetPod(ctx, opts.Podname)
	if err != nil {
		log.Errorf("[CreateContainer] Error during GetPod for %s: %v", opts.Podname, err)
		return nil, err
	}
	log.Infof("[CreateContainer] Creating container with options: %v", opts)

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

	if pod.Favor == scheduler.MEMORY_PRIOR {
		return c.createContainerWithMemoryPrior(ctx, opts)
	} else if pod.Favor == scheduler.CPU_PRIOR {
		return c.createContainerWithCPUPrior(ctx, opts)
	}
	return nil, types.NewDetailedErr(types.ErrBadFaver, pod.Favor)
}

func (c *Calcium) createContainerWithMemoryPrior(ctx context.Context, opts *types.DeployOptions) (chan *types.CreateContainerMessage, error) {
	ch := make(chan *types.CreateContainerMessage)

	// TODO RFC 计算当前 app 部署情况的时候需要保证同一时间只有这个 app 的这个 entrypoint 在跑
	// 因此需要在这里加个全局锁，直到部署完毕才释放
	nodesInfo, err := c.allocResource(ctx, opts, scheduler.MEMORY_PRIOR)
	if err != nil {
		log.Errorf("[createContainerWithMemoryPrior] Error during allocMemoryPodResource with opts %v: %v", opts, err)
		return ch, err
	}

	go func() {
		defer close(ch)
		wg := sync.WaitGroup{}
		wg.Add(len(nodesInfo))
		index := 0
		for _, nodeInfo := range nodesInfo {
			go stats.Client.SendDeployCount(nodeInfo.Deploy)
			go func(nodeInfo types.NodeInfo, index int) {
				defer wg.Done()
				for _, m := range c.doCreateContainerWithMemoryPrior(ctx, nodeInfo, opts, index) {
					ch <- m
				}
			}(nodeInfo, index)
			index += nodeInfo.Deploy
		}
		wg.Wait()
		// 第一次部署的时候就去cache下镜像吧
		go c.cacheImage(ctx, opts.Podname, opts.Image)
	}()

	return ch, nil
}

func (c *Calcium) doCreateContainerWithMemoryPrior(ctx context.Context, nodeInfo types.NodeInfo, opts *types.DeployOptions, index int) []*types.CreateContainerMessage {
	ms := make([]*types.CreateContainerMessage, nodeInfo.Deploy)

	node, err := c.getAndPrepareNode(ctx, opts.Podname, nodeInfo.Name, opts.Image)
	if err != nil {
		log.Errorf("[doCreateContainerWithMemoryPrior] Get and prepare node error %v", err)
		for i := 0; i < nodeInfo.Deploy; i++ {
			ms[i] = &types.CreateContainerMessage{Error: err}
			if err := c.store.UpdateNodeResource(ctx, opts.Podname, nodeInfo.Name, types.CPUMap{}, opts.Memory, "+"); err != nil {
				log.Errorf("[doCreateContainerWithMemoryPrior] reset node memory failed %v", err)
			}
		}
		return ms
	}

	for i := 0; i < nodeInfo.Deploy; i++ {
		// createAndStartContainer will auto cleanup
		ms[i] = c.createAndStartContainer(ctx, i+index, node, opts, nil, scheduler.MEMORY_PRIOR)
		if !ms[i].Success {
			log.Errorf("[doCreateContainerWithMemoryPrior] Error when create and start a container, %v", ms[i].Error)
			continue
		}
		log.Debugf("[doCreateContainerWithMemoryPrior] create container success %s", ms[i].ContainerID)
	}

	return ms
}

func (c *Calcium) createContainerWithCPUPrior(ctx context.Context, opts *types.DeployOptions) (chan *types.CreateContainerMessage, error) {
	ch := make(chan *types.CreateContainerMessage)
	nodesInfo, err := c.allocResource(ctx, opts, scheduler.CPU_PRIOR)
	if err != nil {
		log.Errorf("[createContainerWithCPUPrior] Error during allocCPUPodResource with opts %v: %v", opts, err)
		return ch, err
	}

	if len(nodesInfo) == 0 {
		return ch, types.ErrInsufficientRes
	}

	go func() {
		defer close(ch)
		wg := sync.WaitGroup{}
		wg.Add(len(nodesInfo))
		index := 0

		// do deployment
		for _, nodeInfo := range nodesInfo {
			planCount := len(nodeInfo.CPUPlan)
			go stats.Client.SendDeployCount(planCount)
			go func(nodename string, cpuMap []types.CPUMap, index int) {
				defer wg.Done()
				for _, m := range c.doCreateContainerWithCPUPrior(ctx, nodename, cpuMap, opts, index) {
					ch <- m
				}
			}(nodeInfo.Name, nodeInfo.CPUPlan, index)
			index += planCount
		}

		wg.Wait()
		// 第一次部署的时候就去cache下镜像吧
		go c.cacheImage(ctx, opts.Podname, opts.Image)
	}()

	return ch, nil
}

func (c *Calcium) doCreateContainerWithCPUPrior(ctx context.Context, nodename string, cpuMap []types.CPUMap, opts *types.DeployOptions, index int) []*types.CreateContainerMessage {
	deployCount := len(cpuMap)
	ms := make([]*types.CreateContainerMessage, deployCount)

	node, err := c.getAndPrepareNode(ctx, opts.Podname, nodename, opts.Image)
	if err != nil {
		log.Errorf("[doCreateContainerWithCPUPrior] Get and prepare node error %v", err)
		for i := 0; i < deployCount; i++ {
			ms[i] = &types.CreateContainerMessage{Error: err}
			if err := c.store.UpdateNodeResource(ctx, opts.Podname, nodename, cpuMap[i], opts.Memory, "+"); err != nil {
				log.Errorf("[doCreateContainerWithCPUPrior] update node CPU failed %v", err)
			}
		}
		return ms
	}

	for i, quota := range cpuMap {
		// createAndStartContainer will auto cleanup
		ms[i] = c.createAndStartContainer(ctx, i+index, node, opts, quota, scheduler.CPU_PRIOR)
		if !ms[i].Success {
			log.Errorf("[doCreateContainerWithCPUPrior] Error when create and start a container, %v", ms[i].Error)
			continue
		}
		log.Debugf("[doCreateContainerWithCPUPrior] create container success %s", ms[i].ContainerID)
	}

	return ms
}

func (c *Calcium) makeContainerOptions(index int, quota types.CPUMap, opts *types.DeployOptions, node *types.Node, favor string) (
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
	log.Debugf("[makeContainerOptions] App %s will bind %v", opts.Name, binds)

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

	// labels
	// basic labels, and set meta in opts to labels
	containerLabels := map[string]string{
		"ERU":     "1",
		"version": utils.GetVersion(opts.Image),
	}

	// 发布端口
	containerLabels["publish"] = strings.Join(opts.Entrypoint.Publish, ",")
	// 健康检查
	if entry.HealthCheck != nil {
		containerLabels["healthcheck"] = "1"
		containerLabels["healthcheck_tcp"] = strings.Join(entry.HealthCheck.TCPPorts, ",")
		containerLabels["healthcheck_http"] = entry.HealthCheck.HTTPPort
		containerLabels["healthcheck_url"] = entry.HealthCheck.HTTPURL
		containerLabels["healthcheck_code"] = strconv.Itoa(entry.HealthCheck.HTTPCode)
	}

	// 接下来是meta
	for key, value := range opts.Meta {
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

	var resource enginecontainer.Resources
	if favor == scheduler.CPU_PRIOR {
		resource = makeCPUPriorSetting(c.config.Scheduler.ShareBase, quota, opts.Memory)
	} else if favor == scheduler.MEMORY_PRIOR {
		resource = makeMemoryPriorSetting(opts.Memory, opts.CPUQuota)
	} else {
		return nil, nil, nil, "", types.NewDetailedErr(types.ErrBadFaver, favor)
	}
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
	}
	networkConfig := &enginenetwork.NetworkingConfig{}
	return config, hostConfig, networkConfig, containerName, nil
}

func (c *Calcium) getAndPrepareNode(ctx context.Context, podname, nodename, image string) (*types.Node, error) {
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

func (c *Calcium) createAndStartContainer(
	ctx context.Context,
	no int, node *types.Node,
	opts *types.DeployOptions,
	cpu types.CPUMap, typ string,
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
	}
	createContainerMessage := &types.CreateContainerMessage{
		Podname:  container.Podname,
		Nodename: container.Nodename,
		Success:  false,
		CPU:      cpu,
		Quota:    opts.CPUQuota,
		Memory:   opts.Memory,
		Publish:  map[string]string{},
	}

	defer func() {
		if !createContainerMessage.Success {
			if err := c.removeOneContainer(ctx, container); err != nil {
				log.Errorf("[createAndStartContainer] create and start container failed, and remove it failed also %v", err)
			}
		}
	}()

	// get config
	config, hostConfig, networkConfig, containerName, err := c.makeContainerOptions(no, cpu, opts, node, typ)
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
		for dst, byteData := range opts.Data {
			path := filepath.Dir(dst)
			filename := filepath.Base(dst)
			log.Debugf("[createAndStartContainer] Copy file %s to dir %s", filename, path)
			r, err := createTarFileBuffer(filename, byteData)
			if err != nil {
				createContainerMessage.Error = err
				return createContainerMessage
			}
			if err = node.Engine.CopyToContainer(ctx, containerCreated.ID, path, r, enginetypes.CopyToContainerOptions{AllowOverwriteDirWithFile: true, CopyUIDGID: true}); err != nil {
				createContainerMessage.Error = err
				return createContainerMessage
			}
		}
	}

	if err = node.Engine.ContainerStart(ctx, containerCreated.ID, enginetypes.ContainerStartOptions{}); err != nil {
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
		createContainerMessage.Hook = []byte{}
		for _, cmd := range opts.Entrypoint.Hook.AfterStart {
			output, err := execuateInside(ctx, node.Engine, container.ID, cmd, opts.User, opts.Env, container.Privileged)
			if err != nil {
				if opts.Entrypoint.Hook.Force {
					createContainerMessage.Error = err
					return createContainerMessage
				}
				createContainerMessage.Hook = append(createContainerMessage.Hook, []byte(err.Error())...)
				continue
			}
			createContainerMessage.Hook = append(createContainerMessage.Hook, output...)
		}
	}

	// get ips
	if containerAlived.NetworkSettings != nil {
		for nn, ns := range containerAlived.NetworkSettings.Networks {
			ip := ns.IPAddress
			if enginecontainer.NetworkMode(nn).IsHost() {
				ip = node.GetIP()
			}

			data := []string{}
			for _, port := range opts.Entrypoint.Publish {
				data = append(data, fmt.Sprintf("%s:%s", ip, port))
			}
			if len(data) == 0 {
				createContainerMessage.Publish[nn] = ip
			} else {
				createContainerMessage.Publish[nn] = strings.Join(data, ",")
			}
		}
	}

	createContainerMessage.Success = true
	return createContainerMessage
}
