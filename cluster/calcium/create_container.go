package calcium

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	log "github.com/Sirupsen/logrus"
	enginetypes "github.com/docker/docker/api/types"
	enginecontainer "github.com/docker/docker/api/types/container"
	enginenetwork "github.com/docker/docker/api/types/network"
	engineslice "github.com/docker/docker/api/types/strslice"
	"github.com/docker/go-units"
	"gitlab.ricebook.net/platform/core/stats"
	"gitlab.ricebook.net/platform/core/types"
	"gitlab.ricebook.net/platform/core/utils"
)

const (
	RESTART_ALWAYS   = "always"
	MEMORY_LOW_LIMIT = 4194304
)

// Create Container
// Use specs and options to create
// TODO what about networks?
func (c *calcium) CreateContainer(specs types.Specs, opts *types.DeployOptions) (chan *types.CreateContainerMessage, error) {
	pod, err := c.store.GetPod(opts.Podname)
	if err != nil {
		log.Errorf("Error during GetPod for %s: %v", opts.Podname, err)
		return nil, err
	}
	if pod.Favor == types.CPU_PRIOR {
		return c.createContainerWithCPUPrior(specs, opts)
	}
	log.Infof("[CreateContainer] Creating container with options: %v", opts)
	return c.createContainerWithMemoryPrior(specs, opts)
}

func (c *calcium) createContainerWithMemoryPrior(specs types.Specs, opts *types.DeployOptions) (chan *types.CreateContainerMessage, error) {
	ch := make(chan *types.CreateContainerMessage)
	if opts.Memory < MEMORY_LOW_LIMIT { // 4194304 Byte = 4 MB, docker 创建容器的内存最低标准
		return ch, fmt.Errorf("Minimum memory limit allowed is 4MB, got %d", opts.Memory)
	}
	if opts.Count <= 0 { // Count 要大于0
		return ch, fmt.Errorf("Count must be positive, got %d", opts.Count)
	}

	// TODO RFC 计算当前 app 部署情况的时候需要保证同一时间只有这个 app 的这个 entrypoint 在跑
	// 因此需要在这里加个全局锁，直到部署完毕才释放
	nodesInfo, err := c.allocMemoryPodResource(opts)
	if err != nil {
		log.Errorf("Error during allocMemoryPodResource with opts %v: %v", opts, err)
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
				for _, m := range c.doCreateContainerWithMemoryPrior(nodeInfo, specs, opts, index) {
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

func (c *calcium) removeMemoryPodFailedContainer(id string, node *types.Node, nodeInfo types.NodeInfo, opts *types.DeployOptions) {
	defer c.store.UpdateNodeMem(opts.Podname, nodeInfo.Name, opts.Memory, "+")
	if err := node.Engine.ContainerRemove(context.Background(), id, enginetypes.ContainerRemoveOptions{}); err != nil {
		log.Errorf("[RemoveMemoryPodFailedContainer] Error during remove failed container %v", err)
	}
}

func (c *calcium) doCreateContainerWithMemoryPrior(nodeInfo types.NodeInfo, specs types.Specs, opts *types.DeployOptions, index int) []*types.CreateContainerMessage {
	ms := make([]*types.CreateContainerMessage, nodeInfo.Deploy)
	var i int
	for i = 0; i < nodeInfo.Deploy; i++ {
		ms[i] = &types.CreateContainerMessage{}
	}

	node, err := c.GetNode(opts.Podname, nodeInfo.Name)
	if err != nil {
		log.Errorf("Error during GetNode for %s, %s: %v", opts.Podname, nodeInfo.Name, err)
		return ms
	}

	if err := pullImage(node, opts.Image); err != nil {
		log.Errorf("Error during pullImage %s for %s: %v", opts.Image, nodeInfo.Name, err)
		for i := 0; i < nodeInfo.Deploy; i++ {
			ms[i].Error = err.Error()
		}
		return ms
	}

	for i = 0; i < nodeInfo.Deploy; i++ {
		config, hostConfig, networkConfig, containerName, err := c.makeContainerOptions(i+index, nil, specs, opts, node, types.MEMORY_PRIOR)
		ms[i].ContainerName = containerName
		ms[i].Podname = opts.Podname
		ms[i].Nodename = node.Name
		ms[i].Memory = opts.Memory
		if err != nil {
			log.Errorf("Error during makeContainerOptions: %v", err)
			ms[i].Error = err.Error()
			c.store.UpdateNodeMem(opts.Podname, nodeInfo.Name, opts.Memory, "+") // 创建容器失败就要把资源还回去对不对？
			continue
		}

		//create container
		container, err := node.Engine.ContainerCreate(context.Background(), config, hostConfig, networkConfig, containerName)
		if err != nil {
			log.Errorf("[CreateContainerWithMemoryPrior] Error during ContainerCreate, %v", err)
			ms[i].Error = err.Error()
			c.store.UpdateNodeMem(opts.Podname, nodeInfo.Name, opts.Memory, "+")
			continue
		}

		// connect container to network
		// if network manager uses docker plugin, then connect must be called before container starts
		if c.network.Type() == "plugin" {
			ctx := utils.ToDockerContext(node.Engine)
			breaked := false

			// need to ensure all networks are correctly connected
			for networkID, ipv4 := range opts.Networks {
				if err = c.network.ConnectToNetwork(ctx, container.ID, networkID, ipv4); err != nil {
					log.Errorf("[CreateContainerWithMemoryPrior] Error during connecting container %q to network %q, %v", container.ID, networkID, err)
					breaked = true
					c.store.UpdateNodeMem(opts.Podname, nodeInfo.Name, opts.Memory, "+")
					break
				}
			}

			// remove bridge network
			if len(opts.Networks) != 0 {
				if err := c.network.DisconnectFromNetwork(ctx, container.ID, "bridge"); err != nil {
					log.Errorf("[CreateContainerWithMemoryPrior] Error during disconnecting container %q from network %q, %v", container.ID, "bridge", err)
				}
			}

			// if any break occurs, then this container needs to be removed
			if breaked {
				ms[i].Error = err.Error()
				go c.removeMemoryPodFailedContainer(container.ID, node, nodeInfo, opts)
				continue
			}
		}
		if err = node.Engine.ContainerStart(context.Background(), container.ID, enginetypes.ContainerStartOptions{}); err != nil {
			log.Errorf("[CreateContainerWithMemoryPrior] Error during ContainerStart, %v", err)
			ms[i].Error = err.Error()
			go c.removeMemoryPodFailedContainer(container.ID, node, nodeInfo, opts)
			continue
		}

		// TODO
		// if network manager uses our own, then connect must be called after container starts
		// here
		info, err := node.Engine.ContainerInspect(context.Background(), container.ID)
		if err != nil {
			log.Errorf("[CreateContainerWithMemoryPrior] Error during ContainerInspect, %v", err)
			ms[i].Error = err.Error()
			c.store.UpdateNodeMem(opts.Podname, nodeInfo.Name, opts.Memory, "+")
			continue
		}
		ms[i].ContainerID = info.ID

		// after start
		if err := runExec(node.Engine, info, AFTER_START); err != nil {
			log.Errorf("[CreateContainerWithMemoryPrior] Run exec at %s error: %v", AFTER_START, err)
		}

		_, err = c.store.AddContainer(info.ID, opts.Podname, node.Name, containerName, nil, opts.Memory)
		if err != nil {
			log.Errorf("[CreateContainerWithMemoryPrior] Error during store etcd data %v", err)
			ms[i].Error = err.Error()
			go c.removeMemoryPodFailedContainer(container.ID, node, nodeInfo, opts)
			continue
		}
		log.Debugf("[doCreateContainer] create container success %s", info.ID)
		ms[i].Success = true
	}

	go func(opts *types.DeployOptions) {
		cpuandmem, _, err := c.getCPUAndMem(opts.Podname, opts.Nodename, 1.0)
		if err != nil {
			log.Errorf("Get cpu and mem stats failed %v", err)
			return
		}
		stats.Client.SendMemCap(cpuandmem, false)
	}(opts)

	return ms
}

func (c *calcium) createContainerWithCPUPrior(specs types.Specs, opts *types.DeployOptions) (chan *types.CreateContainerMessage, error) {
	ch := make(chan *types.CreateContainerMessage)
	result, err := c.allocCPUPodResource(opts)
	if err != nil {
		log.Errorf("Error during allocCPUPodResource with opts %v: %v", opts, err)
		return ch, err
	}

	if len(result) == 0 {
		return ch, fmt.Errorf("[CreateContainerWithCPUPrior] Not enough resource to create container")
	}

	// FIXME check total count in case scheduler error
	// FIXME ??? why

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
				for _, m := range c.doCreateContainerWithCPUPrior(nodeName, cpuMap, specs, opts, index) {
					ch <- m
				}
			}(nodeName, cpuMap, index)
			index += len(cpuMap)
		}

		wg.Wait()
	}()

	return ch, nil
}

func (c *calcium) removeCPUPodFailedContainer(id string, node *types.Node, quota types.CPUMap) {
	defer c.releaseQuota(node, quota)
	if err := node.Engine.ContainerRemove(context.Background(), id, enginetypes.ContainerRemoveOptions{}); err != nil {
		log.Errorf("[RemoveCPUPodFailedContainer] Error during remove failed container %v", err)
	}
}

func (c *calcium) doCreateContainerWithCPUPrior(nodeName string, cpuMap []types.CPUMap, specs types.Specs, opts *types.DeployOptions, index int) []*types.CreateContainerMessage {
	deployCount := len(cpuMap)
	ms := make([]*types.CreateContainerMessage, deployCount)
	for i := 0; i < deployCount; i++ {
		ms[i] = &types.CreateContainerMessage{}
	}

	node, err := c.GetNode(opts.Podname, nodeName)
	if err != nil {
		log.Errorf("[CreateContainerWithCPUPrior] Get node error %v", err)
		return ms
	}

	if err := pullImage(node, opts.Image); err != nil {
		log.Errorf("Error during pullImage: %v", err)
		for i := 0; i < deployCount; i++ {
			ms[i].Error = err.Error()
		}
		return ms
	}

	for i, quota := range cpuMap {
		// create options
		config, hostConfig, networkConfig, containerName, err := c.makeContainerOptions(i+index, quota, specs, opts, node, types.CPU_PRIOR)
		ms[i].ContainerName = containerName
		ms[i].Podname = opts.Podname
		ms[i].Nodename = node.Name
		ms[i].Memory = opts.Memory
		if err != nil {
			log.Errorf("Error during makeContainerOptions: %v", err)
			ms[i].Error = err.Error()
			c.releaseQuota(node, quota)
			continue
		}

		// create container
		container, err := node.Engine.ContainerCreate(context.Background(), config, hostConfig, networkConfig, containerName)
		if err != nil {
			log.Errorf("[CreateContainerWithCPUPrior] Error when creating container, %v", err)
			ms[i].Error = err.Error()
			c.releaseQuota(node, quota)
			continue
		}

		// connect container to network
		// if network manager uses docker plugin, then connect must be called before container starts
		if c.network.Type() == "plugin" {
			ctx := utils.ToDockerContext(node.Engine)
			breaked := false

			// need to ensure all networks are correctly connected
			for networkID, ipv4 := range opts.Networks {
				if err = c.network.ConnectToNetwork(ctx, container.ID, networkID, ipv4); err != nil {
					log.Errorf("[CreateContainerWithCPUPrior] Error when connecting container %q to network %q, %v", container.ID, networkID, err)
					breaked = true
					c.releaseQuota(node, quota)
					break
				}
			}

			// remove bridge network
			// only when user defined networks is given
			if len(opts.Networks) != 0 {
				if err := c.network.DisconnectFromNetwork(ctx, container.ID, "bridge"); err != nil {
					log.Errorf("[CreateContainerWithCPUPrior] Error when disconnecting container %q from network %q, %v", container.ID, "bridge", err)
				}
			}

			// if any break occurs, then this container needs to be removed
			if breaked {
				ms[i].Error = err.Error()
				go c.removeCPUPodFailedContainer(container.ID, node, quota)
				continue
			}
		}
		if err = node.Engine.ContainerStart(context.Background(), container.ID, enginetypes.ContainerStartOptions{}); err != nil {
			log.Errorf("[CreateContainerWithCPUPrior] Error when starting container, %v", err)
			ms[i].Error = err.Error()
			go c.removeCPUPodFailedContainer(container.ID, node, quota)
			continue
		}

		// TODO
		// if network manager uses our own, then connect must be called after container starts
		// here
		info, err := node.Engine.ContainerInspect(context.Background(), container.ID)
		if err != nil {
			log.Errorf("[CreateContainerWithCPUPrior] Error when inspecting container, %v", err)
			ms[i].Error = err.Error()
			c.releaseQuota(node, quota)
			continue
		}
		ms[i].ContainerID = info.ID

		// after start
		if err := runExec(node.Engine, info, AFTER_START); err != nil {
			log.Errorf("[CreateContainerWithCPUPrior] Run exec at %s error: %v", AFTER_START, err)
		}

		if _, err = c.store.AddContainer(info.ID, opts.Podname, node.Name, containerName, quota, opts.Memory); err != nil {
			log.Errorf("[CreateContainerWithCPUPrior] Error during store etcd data %v", err)
			ms[i].Error = err.Error()
			go c.removeCPUPodFailedContainer(container.ID, node, quota)
			continue
		}
		log.Debugf("[doCreateContainer] create container success %s", info.ID)
		ms[i].Success = true
	}

	return ms
}

// When deploy on a public host
// quota is set to 0
// no need to update this to etcd (save 1 time write on etcd)
func (c *calcium) releaseQuota(node *types.Node, quota types.CPUMap) {
	if quota.Total() == 0 {
		log.Debug("cpu quota is zero: %f", quota)
		return
	}
	c.store.UpdateNodeCPU(node.Podname, node.Name, quota, "+")
}

func (c *calcium) makeContainerOptions(index int, quota types.CPUMap, specs types.Specs, opts *types.DeployOptions, node *types.Node, favor string) (
	*enginecontainer.Config,
	*enginecontainer.HostConfig,
	*enginenetwork.NetworkingConfig,
	string,
	error) {

	entry, ok := specs.Entrypoints[opts.Entrypoint]
	if !ok {
		err := fmt.Errorf("Entrypoint %q not found in image %q", opts.Entrypoint, opts.Image)
		log.Errorf("Error during makeContainerOptions: %v", err)
		return nil, nil, nil, "", err
	}

	user := specs.Appname
	// 如果是升级或者是raw, 就用root
	if entry.Privileged != "" || opts.Raw {
		user = "root"
	}
	// command and user
	slices := utils.MakeCommandLineArgs(entry.Command + " " + opts.ExtraArgs)
	cmd := engineslice.StrSlice(slices)

	// env
	nodeIP := node.GetIP()
	env := append(opts.Env, fmt.Sprintf("APP_NAME=%s", specs.Appname))
	env = append(env, fmt.Sprintf("ERU_POD=%s", opts.Podname))
	env = append(env, fmt.Sprintf("ERU_NODE_IP=%s", nodeIP))
	env = append(env, fmt.Sprintf("ERU_NODE_NAME=%s", node.Name))
	env = append(env, fmt.Sprintf("ERU_ZONE=%s", c.config.Zone))
	env = append(env, fmt.Sprintf("APPDIR=%s", filepath.Join(c.config.AppDir, specs.Appname)))
	env = append(env, fmt.Sprintf("ERU_CONTAINER_NO=%d", index))
	env = append(env, fmt.Sprintf("ERU_MEMORY=%d", opts.Memory))

	// mount paths
	binds, volumes := makeMountPaths(specs, c.config)
	log.Debugf("[makeContainerOptions] App %s will bind %v", specs.Appname, binds)

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
			"tag":             fmt.Sprintf("%s {{.ID}}", specs.Appname),
		}
	}

	// working dir 默认是空, 也就是根目录
	// 如果是raw模式, 就以working_dir为主, 默认为空.
	// 如果没有设置working_dir同时又不是raw模式创建, 就用/:appname
	// TODO 是不是要有个白名单或者黑名单之类的
	workingDir := entry.WorkingDir
	if !opts.Raw && workingDir == "" {
		workingDir = strings.TrimRight(c.config.AppDir, "/") + "/" + specs.Appname
	}

	// CapAdd and Privileged
	capAdd := []string{}
	if entry.Privileged == "__super__" {
		capAdd = append(capAdd, "SYS_ADMIN")
	}

	// labels
	// basic labels, and set meta in specs to labels
	containerLabels := map[string]string{
		"ERU":     "1",
		"version": utils.GetVersion(opts.Image),
		"zone":    c.config.Zone,
	}
	// 如果有声明检查的端口就用这个端口
	// 否则还是按照publish出去端口来检查
	if entry.HealthCheckPort != 0 {
		//XXX 随便给个 tcp 吧
		containerLabels["ports"] = fmt.Sprintf("%d/tcp", entry.HealthCheckPort)
	} else {
		ports := []string{}
		for _, port := range entry.Ports {
			ports = append(ports, string(port))
		}
		containerLabels["ports"] = strings.Join(ports, ",")
	}

	// 只要声明了ports，就免费赠送tcp健康检查，如果需要http健康检查，还要单独声明 healthcheck_url
	containerLabels["healthcheck"] = "tcp"
	if entry.HealthCheckUrl != "" {
		containerLabels["healthcheck"] = "http"
		containerLabels["healthcheck_url"] = entry.HealthCheckUrl
		containerLabels["healthcheck_expected_code"] = strconv.Itoa(entry.HealthCheckExpectedCode)
	}

	// 要把after_start和before_stop写进去
	containerLabels[AFTER_START] = entry.AfterStart
	containerLabels[BEFORE_STOP] = entry.BeforeStop
	// 接下来是meta
	for key, value := range specs.Meta {
		containerLabels[key] = value
	}

	// ulimit
	ulimits := []*units.Ulimit{&units.Ulimit{Name: "nofile", Soft: 65535, Hard: 65535}}

	// name
	suffix := utils.RandomString(6)
	containerName := utils.MakeContainerName(specs.Appname, opts.Entrypoint, suffix)

	// network mode
	networkMode := entry.NetworkMode
	if networkMode == "" {
		networkMode = c.config.Docker.NetworkMode
	}

	// dns
	// 如果有给dns就优先用给定的dns.
	// 没有给出dns的时候, 如果设定是用宿主机IP作为dns, 就会把宿主机IP设置过去.
	// 其他情况就是默认值.
	// 哦对, networkMode如果是host也不给dns.
	dns := specs.DNS
	if len(dns) == 0 && c.config.Docker.UseLocalDNS && nodeIP != "" && networkMode != "host" {
		dns = []string{nodeIP}
	}

	config := &enginecontainer.Config{
		Env:             env,
		Cmd:             cmd,
		User:            user,
		Image:           opts.Image,
		Volumes:         volumes,
		WorkingDir:      workingDir,
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
	if restartPolicy == RESTART_ALWAYS {
		maximumRetryCount = 0
	}
	hostConfig := &enginecontainer.HostConfig{
		Binds:         binds,
		DNS:           dns,
		LogConfig:     logConfig,
		NetworkMode:   enginecontainer.NetworkMode(networkMode),
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
		log.Errorf("Error during pulling image %s: %v", image, err)
		return err
	}
	ensureReaderClosed(outStream)
	log.Debugf("[pullImage] Done pulling image %s", image)
	return nil
}
