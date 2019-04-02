package calcium

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/projecteru2/core/cluster"
	enginetypes "github.com/projecteru2/core/engine/types"
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

	// 4194304 Byte = 4 MB, 创建容器的内存最低标准
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
							if err := c.store.UpdateNodeResource(ctx, node, m.CPU, opts.CPUQuota, opts.Memory, store.ActionIncr); err != nil {
								log.Errorf("[doCreateContainer] Reset node %s failed %v", nodeInfo.Name, err)
							}
							c.doUnlock(nodeLock, node.Name)
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
		log.Infof("[doCreateContainerOnNode] create container success %s", ms[i].ContainerID)
	}

	return ms
}

func (c *Calcium) doGetAndPrepareNode(ctx context.Context, podname, nodename, image string) (*types.Node, error) {
	node, err := c.GetNode(ctx, podname, nodename)
	if err != nil {
		return nil, err
	}

	return node, pullImage(ctx, node, image)
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
	config := c.doMakeContainerOptions(no, cpu, opts, node)
	container.Name = config.Name
	createContainerMessage.ContainerName = container.Name

	// create container
	containerCreated, err := node.Engine.VirtualizationCreate(ctx, config)
	if err != nil {
		createContainerMessage.Error = err
		return createContainerMessage
	}
	container.ID = containerCreated.ID
	createContainerMessage.ContainerID = containerCreated.ID

	if err = c.store.AddContainer(ctx, container); err != nil {
		createContainerMessage.Error = err
		return createContainerMessage
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
			if err = node.Engine.VirtualizationCopyTo(ctx, containerCreated.ID, path, f, true, true); err != nil {
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
		createContainerMessage.Hook, err = c.doHook(
			ctx,
			container.ID,
			opts.User,
			opts.Entrypoint.Hook.AfterStart,
			opts.Env,
			opts.Entrypoint.Hook.Force,
			false,
			opts.Entrypoint.Privileged,
			node.Engine,
		)
		if err != nil {
			createContainerMessage.Error = err
			return createContainerMessage
		}
	}

	// get ips
	if containerAlived.Networks != nil {
		createContainerMessage.Publish = utils.MakePublishInfo(containerAlived.Networks, opts.Entrypoint.Publish)
	}

	createContainerMessage.Success = true
	return createContainerMessage
}

func (c *Calcium) doMakeContainerOptions(index int, cpumap types.CPUMap, opts *types.DeployOptions, node *types.Node) *enginetypes.VirtualizationCreateOptions {
	config := &enginetypes.VirtualizationCreateOptions{}
	config.Seq = index
	config.CPU = cpumap.Map()
	config.Quota = opts.CPUQuota
	config.Memory = opts.Memory
	config.SoftLimit = opts.SoftLimit
	entry := opts.Entrypoint

	// 如果有指定用户，用指定用户
	// 没有指定用户，用镜像自己的
	// CapAdd and Privileged
	user := opts.User
	config.CapAdd = []string{}
	if entry.Privileged {
		user = root
		config.CapAdd = append(config.CapAdd, "SYS_ADMIN")
	}

	// command and user
	// extra args is dynamically
	slices := utils.MakeCommandLineArgs(fmt.Sprintf("%s %s", entry.Command, opts.ExtraArgs))
	config.Cmd = slices

	// env
	env := append(opts.Env, fmt.Sprintf("APP_NAME=%s", opts.Name))
	env = append(env, fmt.Sprintf("ERU_POD=%s", opts.Podname))
	env = append(env, fmt.Sprintf("ERU_NODE_NAME=%s", node.Name))
	env = append(env, fmt.Sprintf("ERU_CONTAINER_NO=%d", index))
	env = append(env, fmt.Sprintf("ERU_MEMORY=%d", opts.Memory))

	// mount paths
	binds, volumes := makeMountPaths(opts)
	log.Debugf("[doMakeContainerOptions] App %s will bind %v", opts.Name, binds)

	// log config
	// 默认是配置里的driver, 如果entrypoint有指定就用指定的.
	// 如果用 debug 模式就用默认配置的
	logType := c.config.Docker.Log.Type
	logConfig := c.config.Docker.Log.Config
	if logConfig == nil {
		logConfig = map[string]string{}
	}
	logConfig["tag"] = fmt.Sprintf("%s {{.ID}}", opts.Name)
	if entry.Log != nil && !opts.Debug {
		logType = entry.Log.Type
		logConfig = entry.Log.Config
	}
	config.LogType = logType
	config.LogConfig = logConfig

	// basic labels, bind to EruMeta
	config.Labels = map[string]string{
		cluster.ERUMark: "1",
		cluster.ERUMeta: utils.EncodeMetaInLabel(&types.EruMeta{
			Publish:     opts.Entrypoint.Publish,
			HealthCheck: entry.HealthCheck,
		}),
	}

	// 接下来是meta
	for key, value := range opts.Labels {
		config.Labels[key] = value
	}

	// ulimit
	config.Ulimits = map[string]*enginetypes.VirtualizationUlimits{
		"nofile": &enginetypes.VirtualizationUlimits{Soft: 65535, Hard: 65535},
	}

	// name
	suffix := utils.RandomString(6)
	config.Name = utils.MakeContainerName(opts.Name, opts.Entrypoint.Name, suffix)

	// network mode
	// network mode 和 networks 互斥
	// 没有 networks 的时候用 networkmode 的值
	// 有 networks 的时候一律用 none 作为默认 mode
	config.Network = opts.NetworkMode
	config.Networks = opts.Networks
	if len(opts.Networks) > 0 {
		for name := range opts.Networks {
			config.Network = name
			break
		}
	} else if config.Network == "" {
		config.Network = c.config.Docker.NetworkMode
	}

	// dns
	config.DNS = opts.DNS
	// sysctls
	config.Sysctl = entry.Sysctls

	// restart
	config.RestartPolicy = entry.RestartPolicy
	config.RestartRetryCount = 3
	if config.RestartPolicy == restartAlways {
		config.RestartRetryCount = 0
	}

	// general
	config.User = user
	config.Image = opts.Image
	config.WorkingDir = entry.Dir
	config.Stdin = opts.OpenStdin
	config.Privileged = entry.Privileged
	config.Env = env
	config.Hosts = opts.ExtraHosts
	config.Publish = entry.Publish
	config.NetworkDisabled = false
	config.Binds = binds
	config.Volumes = volumes

	return config
}
