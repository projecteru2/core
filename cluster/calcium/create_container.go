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
	if pod.Favor == types.CPU_PRIOR {
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
	var i int
	for i = 0; i < nodeInfo.Deploy; i++ {
		ms[i] = &types.CreateContainerMessage{}
	}

	node, err := c.getAndPrepareNode(opts.Podname, nodeInfo.Name, opts.Image)
	if err != nil {
		log.Errorf("[doCreateContainerWithCPUPrior] Get and prepare node error %v", err)
		for i := 0; i < nodeInfo.Deploy; i++ {
			ms[i].Error = err.Error()
		}
		return ms
	}

	for i = 0; i < nodeInfo.Deploy; i++ {
		container, err := c.createAndStartContainer(i+index, node, opts, nil, types.MEMORY_PRIOR)
		ms[i].Podname = container.Podname
		ms[i].Nodename = container.Nodename
		ms[i].ContainerName = container.Name
		ms[i].ContainerID = container.ID
		ms[i].Memory = container.Memory
		ms[i].Publish = container.Publish
		ms[i].Success = true
		if err != nil {
			ms[i].Success = false
			ms[i].Error = err.Error()
			c.store.UpdateNodeMem(opts.Podname, nodeInfo.Name, opts.Memory, "+") // 创建容器失败就要把资源还回去对不对？
			// clean up
			if container.ID != "" {
				if err := node.Engine.ContainerRemove(context.Background(), container.ID, enginetypes.ContainerRemoveOptions{}); err != nil {
					log.Errorf("[doCreateContainerWithMemoryPrior] Error during remove failed container %v", err)
				}
			}
			log.Errorf("[doCreateContainerWithMemoryPrior] Error when create and start a container, %v", err)
			continue
		}
		log.Debugf("[doCreateContainerWithMemoryPrior] create container success %s", container.ID)
	}

	go func(opts *types.DeployOptions) {
		cpuandmem, _, err := c.getCPUAndMem(opts.Podname, opts.Nodename, 1.0)
		if err != nil {
			log.Errorf("[doCreateContainerWithMemoryPrior] Get cpu and mem stats failed %v", err)
			return
		}
		stats.Client.SendMemCap(cpuandmem, false)
	}(opts)

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
	for i := 0; i < deployCount; i++ {
		ms[i] = &types.CreateContainerMessage{}
	}

	node, err := c.getAndPrepareNode(opts.Podname, nodeName, opts.Image)
	if err != nil {
		log.Errorf("[doCreateContainerWithCPUPrior] Get and prepare node error %v", err)
		for i := 0; i < deployCount; i++ {
			ms[i].Error = err.Error()
		}
		return ms
	}

	for i, quota := range cpuMap {
		container, err := c.createAndStartContainer(i+index, node, opts, quota, types.CPU_PRIOR)
		ms[i].Podname = container.Podname
		ms[i].Nodename = container.Nodename
		ms[i].ContainerName = container.Name
		ms[i].ContainerID = container.ID
		ms[i].CPU = container.CPU
		ms[i].Publish = container.Publish
		ms[i].Success = true
		if err != nil {
			ms[i].Success = false
			ms[i].Error = err.Error()
			if quota.Total() != 0 {
				c.store.UpdateNodeCPU(opts.Podname, node.Name, quota, "+")
			}
			// clean up
			if container.ID != "" {
				if err := node.Engine.ContainerRemove(context.Background(), container.ID, enginetypes.ContainerRemoveOptions{}); err != nil {
					log.Errorf("[doCreateContainerWithCPUPrior] Error during remove failed container %v", err)
				}
			}
			log.Errorf("[doCreateContainerWithCPUPrior] Error when create and start a container, %v", err)
			continue
		}
		log.Debugf("[doCreateContainerWithCPUPrior] create container success %s", container.ID)
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
	// 没有指定用户，用 Name
	// 如果有 privileged，强制 root
	user := opts.Name
	if opts.User != "" {
		user = opts.User
	}
	if entry.Privileged != "" {
		user = root
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
	env = append(env, fmt.Sprintf("ERU_ZONE=%s", c.config.Zone))
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

	// CapAdd and Privileged
	capAdd := []string{}
	if entry.Privileged == "__super__" {
		capAdd = append(capAdd, "SYS_ADMIN")
	}

	// labels
	// basic labels, and set meta in opts to labels
	containerLabels := map[string]string{
		"ERU":     "1",
		"version": utils.GetVersion(opts.Image),
		"zone":    c.config.Zone,
	}

	// 发布端口
	containerLabels["publish"] = strings.Join(utils.DecodePorts(opts.Entrypoint.Publish), ",")
	// 健康检查
	containerLabels["healthcheck_url"] = entry.HealthCheck.URL
	containerLabels["healthcheck_expected_code"] = strconv.Itoa(entry.HealthCheck.Code)
	containerLabels["healthcheck_ports"] = strings.Join(utils.DecodePorts(opts.Entrypoint.HealthCheck.Ports), ",")

	// 要把after_start和before_stop写进去
	containerLabels[afterStart] = entry.Hook.AfterStart
	containerLabels[beforeStop] = entry.Hook.BeforeStop
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
		for name, _ := range opts.Networks {
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
		WorkingDir:      entry.WorkingDir,
		NetworkDisabled: false,
		Labels:          containerLabels,
		OpenStdin:       opts.OpenStdin,
	}

	var resource enginecontainer.Resources
	if favor == types.CPU_PRIOR {
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
		ExtraHosts:    entry.ExtraHosts,
		Privileged:    entry.Privileged != "",
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

func (c *calcium) createAndStartContainer(no int, node *types.Node, opts *types.DeployOptions, quota types.CPUMap, typ string) (*types.Container, error) {
	container := &types.Container{
		Podname:  opts.Podname,
		Nodename: node.Name,
		Publish:  map[string]string{},
		Memory:   opts.Memory,
		CPU:      quota,
		Engine:   node.Engine,
	}

	// get config
	config, hostConfig, networkConfig, containerName, err := c.makeContainerOptions(no, quota, opts, node, typ)
	if err != nil {
		return container, err
	}
	container.Name = containerName

	// create container
	containerCreated, err := node.Engine.ContainerCreate(context.Background(), config, hostConfig, networkConfig, containerName)
	if err != nil {
		return container, err
	}
	container.ID = containerCreated.ID

	// connect container to network
	// if network manager uses docker plugin, then connect must be called before container starts
	// 如果有 networks 的配置，这里的 networkMode 就为 none 了
	if len(opts.Networks) > 0 {
		ctx := utils.ToDockerContext(node.Engine)
		// need to ensure all networks are correctly connected
		for networkID, ipv4 := range opts.Networks {
			if err = c.network.ConnectToNetwork(ctx, containerCreated.ID, networkID, ipv4); err != nil {
				return container, err
			}
		}
	}

	if err = node.Engine.ContainerStart(context.Background(), containerCreated.ID, enginetypes.ContainerStartOptions{}); err != nil {
		return container, err
	}

	containerAlived, err := container.Inspect()
	if err != nil {
		return container, err
	}

	// after start
	if err := runExec(node.Engine, containerAlived, afterStart); err != nil {
		return container, err
	}

	// get ips
	for nn, ns := range containerAlived.NetworkSettings.Networks {
		ip := ns.IPAddress
		if enginecontainer.NetworkMode(nn).IsHost() {
			ip = node.GetIP()
		}

		data := []string{}
		for _, port := range opts.Entrypoint.Publish {
			data = append(data, fmt.Sprintf("%s:%s", ip, port.Port()))
		}
		container.Publish[nn] = strings.Join(data, ",")
	}

	if err = c.store.AddContainer(container); err != nil {
		return container, err
	}

	return container, nil
}
