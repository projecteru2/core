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
	"github.com/projecteru2/core/stats"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

const (
	restartAlways = "always"
	minMemory     = 4194304
)

// Create Container
// Use specs and options to create
// TODO what about networks?
func (c *calcium) CreateContainer(specs types.Specs, opts *types.DeployOptions) (chan *types.CreateContainerMessage, error) {
	pod, err := c.store.GetPod(opts.Podname)
	if err != nil {
		log.Errorf("[CreateContainer] Error during GetPod for %s: %v", opts.Podname, err)
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

func (c *calcium) doCreateContainerWithMemoryPrior(nodeInfo types.NodeInfo, specs types.Specs, opts *types.DeployOptions, index int) []*types.CreateContainerMessage {
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
		container, err := c.createAndStartContainer(i+index, node, opts, specs, nil, types.MEMORY_PRIOR)
		ms[i].Podname = opts.Podname
		ms[i].Nodename = node.Name
		ms[i].ContainerID = container.ID
		ms[i].ContainerName = utils.Tail(container.Name)
		ms[i].Memory = opts.Memory
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

func (c *calcium) createContainerWithCPUPrior(specs types.Specs, opts *types.DeployOptions) (chan *types.CreateContainerMessage, error) {
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

func (c *calcium) doCreateContainerWithCPUPrior(nodeName string, cpuMap []types.CPUMap, specs types.Specs, opts *types.DeployOptions, index int) []*types.CreateContainerMessage {
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
		container, err := c.createAndStartContainer(i+index, node, opts, specs, quota, types.CPU_PRIOR)
		ms[i].Podname = opts.Podname
		ms[i].Nodename = node.Name
		ms[i].ContainerID = container.ID
		ms[i].ContainerName = utils.Tail(container.Name)
		ms[i].CPU = quota
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

func (c *calcium) makeContainerOptions(index int, quota types.CPUMap, specs types.Specs, opts *types.DeployOptions, node *types.Node, favor string) (
	*enginecontainer.Config,
	*enginecontainer.HostConfig,
	*enginenetwork.NetworkingConfig,
	string,
	error) {

	entry, ok := specs.Entrypoints[opts.Entrypoint]
	if !ok {
		err := fmt.Errorf("Entrypoint %q not found in image %q", opts.Entrypoint, opts.Image)
		log.Errorf("[makeContainerOptions] Error during makeContainerOptions: %v", err)
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
	containerLabels[afterStart] = entry.AfterStart
	containerLabels[beforeStop] = entry.BeforeStop
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
	dns := specs.DNS
	if len(dns) == 0 && c.config.Docker.UseLocalDNS && nodeIP != "" && !engineNetworkMode.IsHost() {
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

func (c *calcium) createAndStartContainer(no int, node *types.Node, opts *types.DeployOptions, specs types.Specs, quota types.CPUMap, typ string) (enginetypes.ContainerJSON, error) {
	// get config
	config, hostConfig, networkConfig, containerName, err := c.makeContainerOptions(no, quota, specs, opts, node, typ)
	if err != nil {
		return enginetypes.ContainerJSON{}, err
	}
	// create container
	container, err := node.Engine.ContainerCreate(context.Background(), config, hostConfig, networkConfig, containerName)
	if err != nil {
		return enginetypes.ContainerJSON{}, err
	}

	containerID := container.ID
	containerCreated := enginetypes.ContainerJSON{ContainerJSONBase: &enginetypes.ContainerJSONBase{ID: containerID}}
	// connect container to network
	// if network manager uses docker plugin, then connect must be called before container starts
	// 如果有 networks 的配置，这里的 networkMode 就为 none 了
	if len(opts.Networks) > 0 {
		ctx := utils.ToDockerContext(node.Engine)
		// need to ensure all networks are correctly connected
		for networkID, ipv4 := range opts.Networks {
			if err = c.network.ConnectToNetwork(ctx, containerID, networkID, ipv4); err != nil {
				return containerCreated, err
			}
		}
	}

	if err = node.Engine.ContainerStart(context.Background(), containerID, enginetypes.ContainerStartOptions{}); err != nil {
		return containerCreated, err
	}

	containerAlived, err := node.Engine.ContainerInspect(context.Background(), containerID)
	if err != nil {
		return containerCreated, err
	}

	// after start
	if err := runExec(node.Engine, containerAlived, afterStart); err != nil {
		return containerAlived, err
	}

	if _, err = c.store.AddContainer(containerAlived.ID, opts.Podname, node.Name, containerName, quota, opts.Memory); err != nil {
		return containerAlived, err
	}

	return containerAlived, nil
}
