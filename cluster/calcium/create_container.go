package calcium

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"

	log "github.com/Sirupsen/logrus"
	enginetypes "github.com/docker/docker/api/types"
	enginecontainer "github.com/docker/docker/api/types/container"
	enginenetwork "github.com/docker/docker/api/types/network"
	engineslice "github.com/docker/docker/api/types/strslice"
	"github.com/docker/go-units"
	"github.com/projecteru2/core/scheduler"
	"github.com/projecteru2/core/stats"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

const (
	restartAlways = "always"
	minMemory     = 4194304
	root          = "root"
)

// Create Container
// Use options to create
func (c *calcium) CreateContainer(opts *types.DeployOptions) (chan *types.CreateContainerMessage, error) {
	pod, err := c.store.GetPod(opts.Podname)
	if err != nil {
		log.Errorf("[CreateContainer] Error during GetPod for %s: %v", opts.Podname, err)
		return nil, err
	}
	if pod.Favor == scheduler.CPU_PRIOR {
		return c.createContainerWithCPUPrior(opts)
	}
	log.Infof("[CreateContainer] Creating container with options: %v", opts)
	return c.createContainerWithMemoryPrior(opts)
}

func (c *calcium) createContainerWithMemoryPrior(opts *types.DeployOptions) (chan *types.CreateContainerMessage, error) {
	ch := make(chan *types.CreateContainerMessage)
	if opts.Memory < minMemory { // 4194304 Byte = 4 MB, docker 创建容器的内存最低标准
		return ch, fmt.Errorf("Minimum memory limit allowed is 4MB, got %d", opts.Memory)
	}
	if opts.Count <= 0 { // Count 要大于0
		return ch, fmt.Errorf("Count must be positive, got %d", opts.Count)
	}

	// TODO RFC 计算当前 app 部署情况的时候需要保证同一时间只有这个 app 的这个 entrypoint 在跑
	// 因此需要在这里加个全局锁，直到部署完毕才释放
	nodesInfo, err := c.allocMemoryPodResource(opts)
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
				for _, m := range c.doCreateContainerWithMemoryPrior(nodeInfo, opts, index) {
					ch <- m
				}
			}(nodeInfo, index)
			index += nodeInfo.Deploy
		}
		wg.Wait()

		// 第一次部署的时候就去cache下镜像吧
		go c.cacheImage(opts.Podname, opts.Image)
	}()

	return ch, nil
}

func (c *calcium) doCreateContainerWithMemoryPrior(nodeInfo types.NodeInfo, opts *types.DeployOptions, index int) []*types.CreateContainerMessage {
	ms := make([]*types.CreateContainerMessage, nodeInfo.Deploy)

	node, err := c.getAndPrepareNode(opts.Podname, nodeInfo.Name, opts.Image)
	if err != nil {
		log.Errorf("[doCreateContainerWithCPUPrior] Get and prepare node error %v", err)
		for i := 0; i < nodeInfo.Deploy; i++ {
			ms[i] = &types.CreateContainerMessage{Error: err}
			if err := c.store.UpdateNodeMem(opts.Podname, nodeInfo.Name, opts.Memory, "+"); err != nil {
				log.Errorf("[doCreateContainerWithCPUPrior] reset node memory failed %v", err)
			}
		}
		return ms
	}

	for i := 0; i < nodeInfo.Deploy; i++ {
		// createAndStartContainer will auto cleanup
		ms[i] = c.createAndStartContainer(i+index, node, opts, nil, scheduler.MEMORY_PRIOR)
		if !ms[i].Success {
			log.Errorf("[doCreateContainerWithMemoryPrior] Error when create and start a container, %v", ms[i].Error)
			continue
		}
		log.Debugf("[doCreateContainerWithMemoryPrior] create container success %s", ms[i].ContainerID)
	}

	return ms
}

func (c *calcium) createContainerWithCPUPrior(opts *types.DeployOptions) (chan *types.CreateContainerMessage, error) {
	ch := make(chan *types.CreateContainerMessage)
	result, err := c.allocCPUPodResource(opts)
	if err != nil {
		log.Errorf("[createContainerWithCPUPrior] Error during allocCPUPodResource with opts %v: %v", opts, err)
		return ch, err
	}

	if len(result) == 0 {
		return ch, fmt.Errorf("Not enough resource to create container")
	}

	go func() {
		defer close(ch)
		wg := sync.WaitGroup{}
		wg.Add(len(result))
		index := 0

		// do deployment
		for nodeName, cpuMap := range result {
			go stats.Client.SendDeployCount(len(cpuMap))
			go func(nodeName string, cpuMap []types.CPUMap, index int) {
				defer wg.Done()
				for _, m := range c.doCreateContainerWithCPUPrior(nodeName, cpuMap, opts, index) {
					ch <- m
				}
			}(nodeName, cpuMap, index)
			index += len(cpuMap)
		}

		wg.Wait()
	}()

	return ch, nil
}

func (c *calcium) doCreateContainerWithCPUPrior(nodeName string, cpuMap []types.CPUMap, opts *types.DeployOptions, index int) []*types.CreateContainerMessage {
	deployCount := len(cpuMap)
	ms := make([]*types.CreateContainerMessage, deployCount)

	node, err := c.getAndPrepareNode(opts.Podname, nodeName, opts.Image)
	if err != nil {
		log.Errorf("[doCreateContainerWithCPUPrior] Get and prepare node error %v", err)
		for i := 0; i < deployCount; i++ {
			ms[i] = &types.CreateContainerMessage{Error: err}
			if err := c.store.UpdateNodeCPU(opts.Podname, nodeName, cpuMap[i], "+"); err != nil {
				log.Errorf("[doCreateContainerWithCPUPrior] update node CPU failed %v", err)
			}
		}
		return ms
	}

	for i, quota := range cpuMap {
		// createAndStartContainer will auto cleanup
		ms[i] = c.createAndStartContainer(i+index, node, opts, quota, scheduler.CPU_PRIOR)
		if !ms[i].Success {
			log.Errorf("[doCreateContainerWithCPUPrior] Error when create and start a container, %v", ms[i].Error)
			continue
		}
		log.Debugf("[doCreateContainerWithCPUPrior] create container success %s", ms[i].ContainerID)
	}

	return ms
}

func (c *calcium) makeContainerOptions(index int, quota types.CPUMap, opts *types.DeployOptions, node *types.Node, favor string) (
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
	// 如果是debug模式就用syslog, 拿配置里的syslog配置来发送.
	logDriver := c.config.Docker.LogDriver
	if entry.LogConfig != "" {
		logDriver = entry.LogConfig
	}
	logConfig := enginecontainer.LogConfig{Type: logDriver}
	if opts.Debug {
		logConfig.Type = "syslog"
		logConfig.Config = map[string]string{
			"syslog-address":  c.config.Syslog.Address,
			"syslog-facility": c.config.Syslog.Facility,
			"syslog-format":   c.config.Syslog.Format,
			"tag":             fmt.Sprintf("%s {{.ID}}", opts.Name),
		}
	}

	// labels
	// basic labels, and set meta in opts to labels
	containerLabels := map[string]string{
		"ERU":     "1",
		"version": utils.GetVersion(opts.Image),
	}

	// 发布端口
	containerLabels["publish"] = strings.Join(utils.DecodePorts(opts.Entrypoint.Publish), ",")
	// 健康检查
	containerLabels["healthcheck_url"] = entry.HealthCheck.URL
	containerLabels["healthcheck_expected_code"] = strconv.Itoa(entry.HealthCheck.Code)
	containerLabels["healthcheck_ports"] = strings.Join(utils.DecodePorts(opts.Entrypoint.HealthCheck.Ports), ",")

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
		resource = c.makeCPUPriorSetting(quota)
	} else {
		resource = c.makeMemoryPriorSetting(opts.Memory, opts.CPUQuota)
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
		LogConfig:     logConfig,
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

// Pull an image
// Blocks until it finishes.
func pullImage(node *types.Node, image string) error {
	log.Debugf("[pullImage] Pulling image %s", image)
	if image == "" {
		return fmt.Errorf("Goddamn empty image, WTF?")
	}

	outStream, err := node.Engine.ImagePull(context.Background(), image, enginetypes.ImagePullOptions{})
	if err != nil {
		log.Errorf("[pullImage] Error during pulling image %s: %v", image, err)
		return err
	}
	ensureReaderClosed(outStream)
	log.Debugf("[pullImage] Done pulling image %s", image)
	return nil
}

func (c *calcium) getAndPrepareNode(podname, nodename, image string) (*types.Node, error) {
	node, err := c.GetNode(podname, nodename)
	if err != nil {
		return nil, err
	}

	if err := pullImage(node, image); err != nil {
		return nil, err
	}
	return node, nil
}

func (c *calcium) createAndStartContainer(
	no int, node *types.Node,
	opts *types.DeployOptions,
	quota types.CPUMap, typ string,
) *types.CreateContainerMessage {
	container := &types.Container{
		Podname:    opts.Podname,
		Nodename:   node.Name,
		CPU:        quota,
		Memory:     opts.Memory,
		Hook:       opts.Entrypoint.Hook,
		Privileged: opts.Entrypoint.Privileged,
		Engine:     node.Engine,
	}
	createContainerMessage := &types.CreateContainerMessage{
		Podname:  container.Podname,
		Nodename: container.Nodename,
		Success:  false,
		CPU:      quota,
		Memory:   opts.Memory,
		Publish:  map[string]string{},
	}

	defer func() {
		if !createContainerMessage.Success {
			if err := c.removeOneContainer(container); err != nil {
				log.Errorf("[createAndStartContainer] create and start container failed, and remove it failed also %v", err)
			}
		}
	}()

	// get config
	config, hostConfig, networkConfig, containerName, err := c.makeContainerOptions(no, quota, opts, node, typ)
	if err != nil {
		createContainerMessage.Error = err
		return createContainerMessage
	}
	container.Name = containerName
	createContainerMessage.ContainerName = container.Name

	// create container
	containerCreated, err := node.Engine.ContainerCreate(context.Background(), config, hostConfig, networkConfig, containerName)
	if err != nil {
		createContainerMessage.Error = err
		return createContainerMessage
	}
	container.ID = containerCreated.ID
	createContainerMessage.ContainerID = container.ID

	if err = c.store.AddContainer(container); err != nil {
		createContainerMessage.Error = err
		return createContainerMessage
	}

	// connect container to network
	// if network manager uses docker plugin, then connect must be called before container starts
	// 如果有 networks 的配置，这里的 networkMode 就为 none 了
	if len(opts.Networks) > 0 {
		ctx := utils.ToDockerContext(node.Engine)
		// need to ensure all networks are correctly connected
		for networkID, ipv4 := range opts.Networks {
			if err = c.network.ConnectToNetwork(ctx, containerCreated.ID, networkID, ipv4); err != nil {
				createContainerMessage.Error = err
				return createContainerMessage
			}
		}
	}

	if err = node.Engine.ContainerStart(context.Background(), containerCreated.ID, enginetypes.ContainerStartOptions{}); err != nil {
		createContainerMessage.Error = err
		return createContainerMessage
	}

	containerAlived, err := container.Inspect()
	if err != nil {
		createContainerMessage.Error = err
		return createContainerMessage
	}

	// after start
	if opts.Entrypoint.Hook != nil && len(opts.Entrypoint.Hook.AfterStart) > 0 {
		createContainerMessage.Hook = []byte{}
		for _, cmd := range opts.Entrypoint.Hook.AfterStart {
			output, err := execuateInside(node.Engine, container.ID, cmd, opts.User, opts.Env, container.Privileged)
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
				data = append(data, fmt.Sprintf("%s:%s", ip, port.Port()))
			}
			createContainerMessage.Publish[nn] = strings.Join(data, ",")
		}
	}

	createContainerMessage.Success = true
	return createContainerMessage
}
