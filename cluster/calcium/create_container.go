package calcium

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	enginetypes "github.com/docker/docker/api/types"
	enginecontainer "github.com/docker/docker/api/types/container"
	enginenetwork "github.com/docker/docker/api/types/network"
	engineslice "github.com/docker/docker/api/types/strslice"
	"github.com/docker/go-units"
	"gitlab.ricebook.net/platform/core/types"
	"gitlab.ricebook.net/platform/core/utils"
	"golang.org/x/net/context"
)

// Create Container
// Use specs and options to create
// TODO what about networks?
func (c *calcium) CreateContainer(specs types.Specs, opts *types.DeployOptions) (chan *types.CreateContainerMessage, error) {
	pod, err := c.store.GetPod(opts.Podname)
	if err != nil {
		return nil, err
	}
	if pod.Scheduler == "CPU" {
		return c.createContainerWithScheduler(specs, opts)
	}
	return c.createContainerWithCPUPeriod(specs, opts)
}

func (c *calcium) createContainerWithCPUPeriod(specs types.Specs, opts *types.DeployOptions) (chan *types.CreateContainerMessage, error) {
	ch := make(chan *types.CreateContainerMessage)

	if opts.Memory < 4194304 { // 4194304 Byte = 4 MB, docker 创建容器的内存最低标准
		return ch, fmt.Errorf("Minimum memory limit allowed is 4MB")
	}

	log.Debugf("Deploy options: %v", opts)
	log.Debugf("Deploy specs: %v", specs)
	cpuandmem, _, err := c.getCPUAndMem(opts.Podname, opts.Nodename, 1.0)
	if err != nil {
		return ch, err
	}
	go utils.SendMemCap(cpuandmem, "before-alloc")
	nodesInfo := utils.GetNodesInfo(cpuandmem)

	log.Debugf("Input opts.CPUQuota: %f", opts.CPUQuota)
	cpuQuota := int(opts.CPUQuota * float64(utils.CpuPeriodBase))
	log.Debugf("Tranfered cpuQuota: %d", cpuQuota)
	plan, err := utils.AllocContainerPlan(nodesInfo, cpuQuota, opts.Memory, opts.Count) // 还是以 Bytes 作单位， 不转换了

	if err != nil {
		return ch, err
	}

	go func(specs types.Specs, plan map[string]int, opts *types.DeployOptions) {
		wg := sync.WaitGroup{}
		wg.Add(len(plan))
		for nodename, num := range plan {
			log.Debugf("Outside doCreateContainerWithCPUPeriod: nodename %s, num %d, specs %v, opts %v", nodename, num, specs, opts)
			go func(nodename string, num int, opts *types.DeployOptions) {
				defer wg.Done()
				log.Debugf("Inside doCreateContainerWithCPUPeriod: nodename %s, num %d, specs %v, opts %v", nodename, num, specs, opts)
				for _, m := range c.doCreateContainerWithCPUPeriod(nodename, num, opts.CPUQuota, specs, opts) {
					ch <- m
				}
			}(nodename, num, opts)
		}
		wg.Wait()

		// 第一次部署的时候就去cache下镜像吧
		go c.cacheImage(opts.Podname, opts.Image)
		close(ch)
	}(specs, plan, opts)

	return ch, nil
}

func (c *calcium) doCreateContainerWithCPUPeriod(nodename string, connum int, quota float64, specs types.Specs, opts *types.DeployOptions) []*types.CreateContainerMessage {
	ms := make([]*types.CreateContainerMessage, connum)
	for i := 0; i < len(ms); i++ {
		ms[i] = &types.CreateContainerMessage{}
	}

	// 更新 MemCap
	// memory := opts.Memory / 1024 / 1024
	memoryTotal := opts.Memory * int64(connum)
	c.store.UpdateNodeMem(opts.Podname, nodename, memoryTotal, "-")

	node, err := c.GetNode(opts.Podname, nodename)
	if err != nil {
		return ms
	}

	if err := pullImage(node, opts.Image); err != nil {
		return ms
	}

	for i := 0; i < connum; i++ {
		config, hostConfig, networkConfig, containerName, err := c.makeContainerOptions(nil, specs, opts, "cpuperiod", node)
		ms[i].ContainerName = containerName
		ms[i].Podname = opts.Podname
		ms[i].Nodename = node.Name
		ms[i].Memory = opts.Memory
		if err != nil {
			c.store.UpdateNodeMem(opts.Podname, nodename, opts.Memory, "+") // 创建容器失败就要把资源还回去对不对？
			ms[i].Error = err.Error()
			continue
		}

		//create container
		container, err := node.Engine.ContainerCreate(context.Background(), config, hostConfig, networkConfig, containerName)
		if err != nil {
			log.Errorf("Error during ContainerCreate, %v", err)
			ms[i].Error = err.Error()
			c.store.UpdateNodeMem(opts.Podname, nodename, opts.Memory, "+")
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
					c.store.UpdateNodeMem(opts.Podname, nodename, opts.Memory, "+")
					log.Errorf("Error during connecting container %q to network %q, %v", container.ID, networkID, err)
					breaked = true
					break
				}
			}

			// remove bridge network
			// only when user defined networks is given
			if len(opts.Networks) != 0 {
				if err := c.network.DisconnectFromNetwork(ctx, container.ID, "bridge"); err != nil {
					log.Errorf("Error during disconnecting container %q from network %q, %v", container.ID, "bridge", err)
				}
			}

			// if any break occurs, then this container needs to be removed
			if breaked {
				ms[i].Error = err.Error()
				c.store.UpdateNodeMem(opts.Podname, nodename, opts.Memory, "+")
				go node.Engine.ContainerRemove(context.Background(), container.ID, enginetypes.ContainerRemoveOptions{})
				continue
			}
		}

		err = node.Engine.ContainerStart(context.Background(), container.ID, enginetypes.ContainerStartOptions{})
		if err != nil {
			log.Errorf("Error during ContainerStart, %v", err)
			ms[i].Error = err.Error()
			c.store.UpdateNodeMem(opts.Podname, nodename, opts.Memory, "+")
			go node.Engine.ContainerRemove(context.Background(), container.ID, enginetypes.ContainerRemoveOptions{})
			continue
		}

		// TODO
		// if network manager uses our own, then connect must be called after container starts
		// here

		info, err := node.Engine.ContainerInspect(context.Background(), container.ID)
		if err != nil {
			log.Errorf("Error during ContainerInspect, %v", err)
			ms[i].Error = err.Error()
			c.store.UpdateNodeMem(opts.Podname, nodename, opts.Memory, "+")
			continue
		}
		ms[i].ContainerID = info.ID

		// after start
		if err := runExec(node.Engine, info, AFTER_START); err != nil {
			log.Errorf("Run exec at %s error: %v", AFTER_START, err)
		}

		_, err = c.store.AddContainer(info.ID, opts.Podname, node.Name, containerName, nil, opts.Memory)
		if err != nil {
			ms[i].Error = err.Error()
			c.store.UpdateNodeMem(opts.Podname, nodename, opts.Memory, "+")
			continue
		}

		ms[i].Success = true

	}

	go func(podname string, nodename string) {
		cpuandmem, _, err := c.getCPUAndMem(podname, nodename, 1.0)
		if err != nil {
			return
		}
		utils.SendMemCap(cpuandmem, "after-alloc")
	}(opts.Podname, opts.Nodename)

	return ms
}

func (c *calcium) createContainerWithScheduler(specs types.Specs, opts *types.DeployOptions) (chan *types.CreateContainerMessage, error) {
	ch := make(chan *types.CreateContainerMessage)

	result, err := c.prepareNodes(opts.Podname, opts.Nodename, opts.CPUQuota, opts.Count)
	if err != nil {
		return ch, err
	}
	if len(result) == 0 {
		return ch, fmt.Errorf("Not enough resource to create container")
	}

	// check total count in case scheduler error
	totalCount := 0
	for _, cores := range result {
		totalCount = totalCount + len(cores)
	}
	if totalCount != opts.Count {
		return ch, fmt.Errorf("Count mismatch (opt.Count %q, total %q), maybe scheduler error?", opts.Count, totalCount)
	}

	go func(plan map[string][]types.CPUMap, opts *types.DeployOptions) {
		wg := sync.WaitGroup{}
		wg.Add(len(result))

		// do deployment
		for nodename, cpumap := range plan {
			go func(nodename string, cpumap []types.CPUMap, opts *types.DeployOptions) {
				defer wg.Done()
				for _, m := range c.doCreateContainerWithScheduler(nodename, cpumap, specs, opts) {
					ch <- m
				}
			}(nodename, cpumap, opts)
		}

		wg.Wait()
		close(ch)
	}(result, opts)

	return ch, nil
}

func makeCPUAndMem(nodes []*types.Node) map[string]types.CPUAndMem {
	r := make(map[string]types.CPUAndMem)
	for _, node := range nodes {
		r[node.Name] = types.CPUAndMem{node.CPU, node.MemCap}
	}
	return r
}

func makeCPUMap(nodes map[string]types.CPUAndMem) map[string]types.CPUMap {
	r := make(map[string]types.CPUMap)
	for key, node := range nodes {
		r[key] = node.CpuMap
	}
	return r
}

func (c *calcium) getCPUAndMem(podname, nodename string, quota float64) (map[string]types.CPUAndMem, []*types.Node, error) {
	result := make(map[string]types.CPUAndMem)
	lock, err := c.store.CreateLock(podname, 30)
	if err != nil {
		return result, nil, err
	}
	if err := lock.Lock(); err != nil {
		return result, nil, err
	}
	defer lock.Unlock()

	// 没有指定nodename, 就从所有的node里面取
	// 指定了就给一个指定的列表
	var nodes []*types.Node
	if nodename == "" {
		nodes, err = c.ListPodNodes(podname, false)
		if err != nil {
			return result, nil, err
		}
	} else {
		n, err := c.GetNode(podname, nodename)
		if err != nil {
			return result, nil, err
		}
		nodes = append(nodes, n)
	}

	if quota == 0 { // 因为要考虑quota=0.5这种需求，所以这里有点麻烦
		nodes = filterNodes(nodes, true)
	} else {
		nodes = filterNodes(nodes, false)
	}

	if len(nodes) == 0 {
		err := fmt.Errorf("No available nodes")
		log.Errorf("Error during getCPUAndMem: %v", err)
		return result, nil, err
	}

	result = makeCPUAndMem(nodes)
	return result, nodes, nil
}

// Prepare nodes for deployment.
// Later if any error occurs, these nodes can be restored.
func (c *calcium) prepareNodes(podname, nodename string, quota float64, num int) (map[string][]types.CPUMap, error) {
	result := make(map[string][]types.CPUMap)
	log.Debugf("Input parameters podname: %s, nodename: %s, quota: %f, num: %d", podname, nodename, quota, num)
	cpuandmem, nodes, err := c.getCPUAndMem(podname, nodename, quota)
	if err != nil {
		return result, err
	}
	cpumap := makeCPUMap(cpuandmem) // 做这个转换，免得改太多
	// use podname as lock key to prevent scheduling on the same node at one time
	log.Debugf("Cpumap: %v", cpumap)
	result, changed, err := c.scheduler.SelectNodes(cpumap, quota, num) // 这个接口统一使用float64了
	log.Debugf("Result: %v, Changed: %v", result, changed)
	if err != nil {
		return result, err
	}

	// if quota is set to 0
	// then no cpu is required
	if quota > 0 {
		// cpus changeded
		// update data to etcd
		// `SelectNodes` reduces count in cpumap
		for _, node := range nodes {
			r, ok := changed[node.Name]
			// 不在changed里说明没有变化
			if ok {
				node.CPU = r
				// ignore error
				c.store.UpdateNode(node)
			}
		}
	}

	return result, err
}

// filter nodes
// public is the flag
func filterNodes(nodes []*types.Node, public bool) []*types.Node {
	rs := []*types.Node{}
	for _, node := range nodes {
		if node.Public == public {
			rs = append(rs, node)
		}
	}
	return rs
}

// Pull an image
// Blocks until it finishes.
func pullImage(node *types.Node, image string) error {
	log.Debugf("Pulling image %s", image)
	if image == "" {
		return fmt.Errorf("Goddamn empty image, WTF?")
	}

	resp, err := node.Engine.ImagePull(context.Background(), image, enginetypes.ImagePullOptions{})
	if err != nil {
		log.Errorf("Error during pulling image %s: %v", image, err)
		return err
	}
	ensureReaderClosed(resp)
	log.Debugf("Done pulling image %s", image)
	return nil
}

func (c *calcium) doCreateContainerWithScheduler(nodename string, cpumap []types.CPUMap, specs types.Specs, opts *types.DeployOptions) []*types.CreateContainerMessage {
	ms := make([]*types.CreateContainerMessage, len(cpumap))
	for i := 0; i < len(ms); i++ {
		ms[i] = &types.CreateContainerMessage{}
	}

	node, err := c.GetNode(opts.Podname, nodename)
	if err != nil {
		log.Errorf("Get node error %v", err)
		return ms
	}

	if err := pullImage(node, opts.Image); err != nil {
		return ms
	}

	for i, quota := range cpumap {
		// create options
		config, hostConfig, networkConfig, containerName, err := c.makeContainerOptions(quota, specs, opts, "scheduler", node)
		ms[i].ContainerName = containerName
		ms[i].Podname = opts.Podname
		ms[i].Nodename = node.Name
		ms[i].Memory = opts.Memory
		if err != nil {
			ms[i].Error = err.Error()
			c.releaseQuota(node, quota)
			continue
		}

		// create container
		container, err := node.Engine.ContainerCreate(context.Background(), config, hostConfig, networkConfig, containerName)
		if err != nil {
			log.Errorf("Error when creating container, %v", err)
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
					log.Errorf("Error when connecting container %q to network %q, %v", container.ID, networkID, err)
					breaked = true
					break
				}
			}

			// remove bridge network
			// only when user defined networks is given
			if len(opts.Networks) != 0 {
				if err := c.network.DisconnectFromNetwork(ctx, container.ID, "bridge"); err != nil {
					log.Errorf("Error when disconnecting container %q from network %q, %v", container.ID, "bridge", err)
				}
			}

			// if any break occurs, then this container needs to be removed
			if breaked {
				ms[i].Error = err.Error()
				c.releaseQuota(node, quota)
				go node.Engine.ContainerRemove(context.Background(), container.ID, enginetypes.ContainerRemoveOptions{})
				continue
			}
		}

		err = node.Engine.ContainerStart(context.Background(), container.ID, enginetypes.ContainerStartOptions{})
		if err != nil {
			log.Errorf("Error when starting container, %v", err)
			ms[i].Error = err.Error()
			c.releaseQuota(node, quota)
			go node.Engine.ContainerRemove(context.Background(), container.ID, enginetypes.ContainerRemoveOptions{})
			continue
		}

		// TODO
		// if network manager uses our own, then connect must be called after container starts
		// here

		info, err := node.Engine.ContainerInspect(context.Background(), container.ID)
		if err != nil {
			log.Errorf("Error when inspecting container, %v", err)
			ms[i].Error = err.Error()
			c.releaseQuota(node, quota)
			continue
		}
		ms[i].ContainerID = info.ID

		// after start
		if err := runExec(node.Engine, info, AFTER_START); err != nil {
			log.Errorf("Run exec at %s error: %v", AFTER_START, err)
		}

		_, err = c.store.AddContainer(info.ID, opts.Podname, node.Name, containerName, quota, opts.Memory)
		if err != nil {
			ms[i].Error = err.Error()
			c.releaseQuota(node, quota)
			continue
		}
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

func (c *calcium) makeContainerOptions(quota map[string]int, specs types.Specs, opts *types.DeployOptions, optionMode string, node *types.Node) (
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
		user = ""
	}
	// command and user
	slices := utils.MakeCommandLineArgs(entry.Command + " " + opts.ExtraArgs)

	// if not use raw to deploy, or use agent as network manager
	// we need to use our own script to start command
	if !opts.Raw && c.network.Type() == "agent" {
		starter, needNetwork := "launcher", "network"
		if entry.Privileged != "" {
			starter = "launcheroot"
		}
		if len(opts.Networks) == 0 {
			needNetwork = "nonetwork"
		}
		slices = append([]string{fmt.Sprintf("/usr/local/bin/%s", starter), needNetwork}, slices...)
		// use default empty value, as root
		user = ""
	}
	cmd := engineslice.StrSlice(slices)

	// calculate CPUShares and CPUSet
	// scheduler won't return more than 1 share quota
	// so the smallest share is the share numerator
	var cpuShares int64
	var cpuSetCpus string
	if optionMode == "scheduler" {
		shareQuota := 10
		labels := []string{}
		for label, share := range quota {
			labels = append(labels, label)
			if share < shareQuota {
				shareQuota = share
			}
		}
		cpuShares = int64(float64(shareQuota) / float64(10) * float64(1024))
		cpuSetCpus = strings.Join(labels, ",")
	}

	// env
	nodeIP := node.GetIP()
	env := append(opts.Env, fmt.Sprintf("APP_NAME=%s", specs.Appname))
	env = append(env, fmt.Sprintf("ERU_POD=%s", opts.Podname))
	env = append(env, fmt.Sprintf("ERU_NODE_IP=%s", nodeIP))
	env = append(env, fmt.Sprintf("ERU_NODE_NAME=%s", node.Name))
	env = append(env, fmt.Sprintf("ERU_ZONE=%s", c.config.Zone))
	env = append(env, fmt.Sprintf("APPDIR=%s", filepath.Join(c.config.AppDir, specs.Appname)))

	// mount paths
	binds, volumes := makeMountPaths(specs, c.config)
	log.Debugf("App %s will bind %v", specs.Appname, binds)

	// log config
	// 默认是配置里的driver, 如果entrypoint有指定json-file就用json-file.
	// 如果是debug模式就用syslog, 拿配置里的syslog配置来发送.
	logDriver := c.config.Docker.LogDriver
	if entry.LogConfig == "json-file" {
		logDriver = "json-file"
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
	if entry.HealthCheckUrl != "" {
		containerLabels["healthcheck"] = "http"
		containerLabels["healthcheck_url"] = entry.HealthCheckUrl
		containerLabels["healthcheck_expected_code"] = strconv.Itoa(entry.HealthCheckExpectedCode)
	} else {
		containerLabels["healthcheck"] = "tcp"
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
	}

	var resource enginecontainer.Resources
	if optionMode == "scheduler" {
		resource = enginecontainer.Resources{
			CPUShares:  cpuShares,
			CpusetCpus: cpuSetCpus,
			Ulimits:    ulimits,
		}
	} else {
		resource = enginecontainer.Resources{
			Memory:     opts.Memory,
			MemorySwap: opts.Memory,
			CPUPeriod:  utils.CpuPeriodBase,
			CPUQuota:   int64(opts.CPUQuota * float64(utils.CpuPeriodBase)),
			Ulimits:    ulimits,
		}
	}

	hostConfig := &enginecontainer.HostConfig{
		Binds:         binds,
		DNS:           dns,
		LogConfig:     logConfig,
		NetworkMode:   enginecontainer.NetworkMode(networkMode),
		RestartPolicy: enginecontainer.RestartPolicy{Name: entry.RestartPolicy, MaximumRetryCount: 3},
		CapAdd:        engineslice.StrSlice(capAdd),
		ExtraHosts:    entry.ExtraHosts,
		Privileged:    entry.Privileged != "",
		Resources:     resource,
	}
	// this is empty because we don't use any plugin for Docker
	// networkConfig := &enginenetwork.NetworkingConfig{
	// 	EndpointsConfig: map[string]*enginenetwork.EndpointSettings{},
	// }

	// for networkID, ipv4 := range opts.Networks {
	// 	networkConfig.EndpointsConfig[networkID] = &enginenetwork.EndpointSettings{
	// 		NetworkID:  networkID,
	// 		IPAMConfig: &enginenetwork.EndpointIPAMConfig{IPv4Address: ipv4},
	// 	}
	// }
	networkConfig := &enginenetwork.NetworkingConfig{}
	return config, hostConfig, networkConfig, containerName, nil
}

// Upgrade containers
// Use image to run these containers, and copy their settings
// Note, if the image is not correct, container will be started incorrectly
// TODO what about networks?
func (c *calcium) UpgradeContainer(ids []string, image string) (chan *types.UpgradeContainerMessage, error) {
	ch := make(chan *types.UpgradeContainerMessage)

	if len(ids) == 0 {
		return ch, fmt.Errorf("No container ids given")
	}

	containers, err := c.GetContainers(ids)
	if err != nil {
		return ch, err
	}

	containerMap := make(map[string][]*types.Container)
	for _, container := range containers {
		containerMap[container.Nodename] = append(containerMap[container.Nodename], container)
	}

	go func() {
		wg := sync.WaitGroup{}
		wg.Add(len(containerMap))

		for _, containers := range containerMap {
			go func(containers []*types.Container, image string) {
				defer wg.Done()

				for _, m := range c.doUpgradeContainer(containers, image) {
					ch <- m
				}
			}(containers, image)

		}

		wg.Wait()
		close(ch)
	}()

	return ch, nil
}

// count user defined networks
// if the name of the network is not "bridge" or "host"
// we treat this as a user defined network
func userDefineNetworks(networks map[string]*enginenetwork.EndpointSettings) map[string]*enginenetwork.EndpointSettings {
	r := make(map[string]*enginenetwork.EndpointSettings)
	for name, network := range networks {
		if name == "bridge" || name == "host" {
			continue
		}
		r[name] = network
	}
	return r
}

// upgrade containers on the same node
func (c *calcium) doUpgradeContainer(containers []*types.Container, image string) []*types.UpgradeContainerMessage {
	ms := make([]*types.UpgradeContainerMessage, len(containers))
	for i := 0; i < len(ms); i++ {
		ms[i] = &types.UpgradeContainerMessage{}
	}

	// TODO ugly
	// use the first container to get node
	// since all containers here must locate on the same node and pod
	t := containers[0]
	node, err := c.GetNode(t.Podname, t.Nodename)
	if err != nil {
		log.Errorf("Get node error %v", err)
		return ms
	}

	// prepare new image
	if err := pullImage(node, image); err != nil {
		return ms
	}

	imagesToDelete := make(map[string]struct{})
	engine := node.Engine

	for i, container := range containers {
		info, err := container.Inspect()
		if err != nil {
			ms[i].Error = err.Error()
			continue
		}

		// have to put it here because later we'll call `makeContainerConfig`
		// which will override this
		// TODO in the future I hope `makeContainerConfig` will make a deep copy of config
		imageToDelete := info.Config.Image

		// before stop old container
		if err := runExec(engine, info, BEFORE_STOP); err != nil {
			log.Errorf("Run exec at %s error: %s", BEFORE_STOP, err.Error())
		}

		// stops the old container
		timeout := 5 * time.Second
		err = engine.ContainerStop(context.Background(), info.ID, &timeout)
		if err != nil {
			ms[i].Error = err.Error()
			continue
		}

		// copy config from old container
		// and of course with a new name
		config, hostConfig, networkConfig, containerName, err := makeContainerConfig(info, image)
		if err != nil {
			ms[i].Error = err.Error()
			engine.ContainerStart(context.Background(), info.ID, enginetypes.ContainerStartOptions{})
			continue
		}

		// create a container with old config and a new name
		newContainer, err := engine.ContainerCreate(context.Background(), config, hostConfig, networkConfig, containerName)
		if err != nil {
			ms[i].Error = err.Error()
			engine.ContainerStart(context.Background(), info.ID, enginetypes.ContainerStartOptions{})
			continue
		}

		// need to disconnect first
		if c.network.Type() == "plugin" {
			ctx := utils.ToDockerContext(engine)
			networks := userDefineNetworks(info.NetworkSettings.Networks)
			// remove new bridge
			// only when user defined networks is given
			if len(networks) != 0 {
				c.network.DisconnectFromNetwork(ctx, newContainer.ID, "bridge")
			}
			// connect to only user defined networks
			for _, endpoint := range networks {
				c.network.DisconnectFromNetwork(ctx, info.ID, endpoint.NetworkID)
				c.network.ConnectToNetwork(ctx, newContainer.ID, endpoint.NetworkID, endpoint.IPAddress)
			}
		}

		// start this new container
		err = engine.ContainerStart(context.Background(), newContainer.ID, enginetypes.ContainerStartOptions{})
		if err != nil {
			go engine.ContainerRemove(context.Background(), newContainer.ID, enginetypes.ContainerRemoveOptions{})
			engine.ContainerStart(context.Background(), info.ID, enginetypes.ContainerStartOptions{})
			ms[i].Error = err.Error()
			continue
		}

		// test if container is correctly started and running
		// if not, restore the old container and remove the new one
		newInfo, err := engine.ContainerInspect(context.Background(), newContainer.ID)
		if err != nil || !newInfo.State.Running {
			ms[i].Error = err.Error()
			// restart the old container
			go engine.ContainerRemove(context.Background(), newContainer.ID, enginetypes.ContainerRemoveOptions{})
			engine.ContainerStart(context.Background(), info.ID, enginetypes.ContainerStartOptions{})
			continue
		}

		// after start
		if err := runExec(engine, newInfo, AFTER_START); err != nil {
			log.Errorf("Run exec at %s error: %v", AFTER_START, err)
		}

		// if so, add a new container in etcd
		_, err = c.store.AddContainer(newInfo.ID, container.Podname, container.Nodename, containerName, container.CPU, container.Memory)
		if err != nil {
			ms[i].Error = err.Error()
			go engine.ContainerRemove(context.Background(), newContainer.ID, enginetypes.ContainerRemoveOptions{})
			engine.ContainerStart(context.Background(), info.ID, enginetypes.ContainerStartOptions{})
			continue
		}

		// remove the old container on node
		rmOpts := enginetypes.ContainerRemoveOptions{
			RemoveVolumes: true,
			Force:         true,
		}
		err = engine.ContainerRemove(context.Background(), info.ID, rmOpts)
		if err != nil {
			ms[i].Error = err.Error()
			continue
		}

		imagesToDelete[imageToDelete] = struct{}{}

		// remove the old container in etcd
		err = c.store.RemoveContainer(info.ID)
		if err != nil {
			ms[i].Error = err.Error()
			continue
		}

		// send back the message
		ms[i].ContainerID = info.ID
		ms[i].NewContainerID = newContainer.ID
		ms[i].NewContainerName = containerName
		ms[i].Success = true
	}

	// clean all the container images
	go func() {
		rmiOpts := enginetypes.ImageRemoveOptions{
			Force:         false,
			PruneChildren: true,
		}
		for image := range imagesToDelete {
			log.Debugf("Try to remove image %q while upgrade container", image)
			engine.ImageRemove(context.Background(), image, rmiOpts)
		}
	}()
	return ms
}
