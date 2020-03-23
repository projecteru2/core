package calcium

import (
	"context"
	"fmt"
	"io"
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
	opts.Normalize()
	opts.ProcessIdent = utils.RandomString(16)
	pod, err := c.store.GetPod(ctx, opts.Podname)
	if err != nil {
		log.Errorf("[CreateContainer %s] Error during GetPod for %s: %v", opts.ProcessIdent, opts.Podname, err)
		return nil, err
	}
	log.Infof("[CreateContainer %s] Creating container with options:", opts.ProcessIdent)
	litter.Dump(opts)
	// Count 要大于0
	if opts.Count <= 0 {
		return nil, types.NewDetailedErr(types.ErrBadCount, opts.Count)
	}
	// 创建时内存不为 0
	if opts.Memory < 0 {
		return nil, types.NewDetailedErr(types.ErrBadMemory, opts.Memory)
	}
	// CPUQuota 也需要大于 0
	if opts.CPUQuota < 0 {
		return nil, types.NewDetailedErr(types.ErrBadCPU, opts.CPUQuota)
	}
	return c.doCreateContainer(ctx, opts, pod)
}

func (c *Calcium) doCreateContainer(ctx context.Context, opts *types.DeployOptions, pod *types.Pod) (chan *types.CreateContainerMessage, error) {
	ch := make(chan *types.CreateContainerMessage)
	// RFC 计算当前 app 部署情况的时候需要保证同一时间只有这个 app 的这个 entrypoint 在跑
	// 因此需要在这里加个全局锁，直到部署完毕才释放
	// 通过 Processing 状态跟踪达成 18 Oct, 2018
	nodesInfo, err := c.doAllocResource(ctx, opts)
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
					if m.Error != nil && m.ContainerID == "" {
						if err := c.withNodeLocked(ctx, nodeInfo.Name, func(node *types.Node) error {
							return c.store.UpdateNodeResource(ctx, node, m.CPU, opts.CPUQuota, opts.Memory, opts.Storage, m.VolumePlan.IntoVolumeMap(), store.ActionIncr)
						}); err != nil {
							log.Errorf("[doCreateContainer] Reset node %s failed %v", nodeInfo.Name, err)
						}
					} else if m.Error != nil && m.ContainerID != "" {
						log.Warnf("[doCreateContainer] Create container failed %v, and container %s not removed", m.Error, m.ContainerID)
					}
					// decr processing count
					if err := c.store.UpdateProcessing(ctx, opts, nodeInfo.Name, nodeInfo.Deploy-i-1); err != nil {
						log.Warnf("[doCreateContainer] Update processing count failed %v", err)
					}
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

	node, err := c.doGetAndPrepareNode(ctx, nodeInfo.Name, opts.Image)
	if err != nil {
		log.Errorf("[doCreateContainerOnNode] Get and prepare node error %v", err)
		for i := 0; i < nodeInfo.Deploy; i++ {
			cpu := types.CPUMap{}
			if len(nodeInfo.CPUPlan) > 0 {
				cpu = nodeInfo.CPUPlan[i]
			}
			volumePlan := types.VolumePlan{}
			if len(nodeInfo.VolumePlans) > 0 {
				volumePlan = nodeInfo.VolumePlans[i]
			}
			ms[i] = &types.CreateContainerMessage{
				Error:      err,
				CPU:        cpu,
				VolumePlan: volumePlan,
			}
		}
		return ms
	}

	for i := 0; i < nodeInfo.Deploy; i++ {
		// createAndStartContainer will auto cleanup
		cpu := types.CPUMap{}
		if len(nodeInfo.CPUPlan) > 0 {
			cpu = nodeInfo.CPUPlan[i]
		}
		volumePlan := types.VolumePlan{}
		if len(nodeInfo.VolumePlans) > 0 {
			volumePlan = nodeInfo.VolumePlans[i]
		}
		ms[i] = c.doCreateAndStartContainer(ctx, i+index, node, opts, cpu, volumePlan)
		if !ms[i].Success {
			log.Errorf("[doCreateContainerOnNode] Error when create and start a container, %v", ms[i].Error)
			continue
		}
		log.Infof("[doCreateContainerOnNode] create container success %s", ms[i].ContainerID)
	}

	return ms
}

func (c *Calcium) doGetAndPrepareNode(ctx context.Context, nodename, image string) (*types.Node, error) {
	node, err := c.GetNode(ctx, nodename)
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
	volumePlan types.VolumePlan,
) *types.CreateContainerMessage {
	container := &types.Container{
		Podname:    opts.Podname,
		Nodename:   node.Name,
		CPU:        cpu,
		Quota:      opts.CPUQuota,
		Memory:     opts.Memory,
		Storage:    opts.Storage,
		Hook:       opts.Entrypoint.Hook,
		Privileged: opts.Entrypoint.Privileged,
		Engine:     node.Engine,
		SoftLimit:  opts.SoftLimit,
		Image:      opts.Image,
		Env:        opts.Env,
		User:       opts.User,
		Volumes:    opts.Volumes,
		VolumePlan: volumePlan,
	}
	createContainerMessage := &types.CreateContainerMessage{
		Podname:    container.Podname,
		Nodename:   container.Nodename,
		Success:    false,
		CPU:        cpu,
		Quota:      opts.CPUQuota,
		Memory:     opts.Memory,
		Storage:    opts.Storage,
		VolumePlan: volumePlan,
		Publish:    map[string][]string{},
	}
	var err error

	defer func() {
		createContainerMessage.Error = err
		if !createContainerMessage.Success && container.ID != "" {
			if err := c.doRemoveContainer(context.Background(), container, true); err != nil {
				log.Errorf("[doCreateAndStartContainer] create and start container failed, and remove it failed also %v", err)
				return
			}
			createContainerMessage.ContainerID = ""
		}
	}()

	// get config
	config := c.doMakeContainerOptions(no, cpu, volumePlan, opts, node)
	container.Name = config.Name
	container.Labels = config.Labels
	createContainerMessage.ContainerName = container.Name

	// create container
	var containerCreated *enginetypes.VirtualizationCreated
	containerCreated, err = node.Engine.VirtualizationCreate(ctx, config)
	if err != nil {
		return createContainerMessage
	}
	container.ID = containerCreated.ID
	createContainerMessage.ContainerID = containerCreated.ID

	// Copy data to container
	if len(opts.Data) > 0 {
		for dst, src := range opts.Data {
			src.Seek(0, io.SeekStart)
			if err = c.doSendFileToContainer(ctx, node.Engine, containerCreated.ID, dst, src, true, true); err != nil {
				return createContainerMessage
			}
		}
	}

	// deal with hook
	if len(opts.AfterCreate) > 0 && container.Hook != nil {
		container.Hook = &types.Hook{
			AfterStart: append(opts.AfterCreate, container.Hook.AfterStart...),
			Force:      container.Hook.Force,
		}
	}

	// start first
	createContainerMessage.Hook, err = c.doStartContainer(ctx, container, opts.IgnoreHook)
	if err != nil {
		return createContainerMessage
	}

	// inspect real meta
	var containerInfo *enginetypes.VirtualizationInfo
	containerInfo, err = container.Inspect(ctx) // 补充静态元数据
	if err != nil {
		return createContainerMessage
	}

	// update meta
	if containerInfo.Networks != nil {
		createContainerMessage.Publish = utils.MakePublishInfo(containerInfo.Networks, opts.Entrypoint.Publish)
	}
	// reset users
	if containerInfo.User != container.User {
		container.User = containerInfo.User
	}
	// reset container.hook
	container.Hook = opts.Entrypoint.Hook

	// store eru container
	if err = c.store.AddContainer(ctx, container); err != nil {
		return createContainerMessage
	}

	// mark success
	createContainerMessage.Success = true
	return createContainerMessage
}

func (c *Calcium) doMakeContainerOptions(index int, cpumap types.CPUMap, volumePlan types.VolumePlan, opts *types.DeployOptions, node *types.Node) *enginetypes.VirtualizationCreateOptions {
	config := &enginetypes.VirtualizationCreateOptions{}
	// general
	config.Seq = index
	config.CPU = cpumap
	config.Quota = opts.CPUQuota
	config.Memory = opts.Memory
	config.Storage = opts.Storage
	config.NUMANode = node.GetNUMANode(cpumap)
	config.SoftLimit = opts.SoftLimit
	config.RawArgs = opts.RawArgs
	config.Lambda = opts.Lambda
	config.User = opts.User
	config.DNS = opts.DNS
	config.Image = opts.Image
	config.Stdin = opts.OpenStdin
	config.Hosts = opts.ExtraHosts
	config.Volumes = opts.Volumes.ApplyPlan(volumePlan).ToStringSlice(false, true)
	config.Debug = opts.Debug
	config.Network = opts.NetworkMode
	config.Networks = opts.Networks

	// entry
	entry := opts.Entrypoint
	config.WorkingDir = entry.Dir
	config.Privileged = entry.Privileged
	config.RestartPolicy = entry.RestartPolicy
	config.Sysctl = entry.Sysctls
	config.Publish = entry.Publish
	if entry.Log != nil {
		config.LogType = entry.Log.Type
		config.LogConfig = entry.Log.Config
	}
	// name
	suffix := utils.RandomString(6)
	config.Name = utils.MakeContainerName(opts.Name, opts.Entrypoint.Name, suffix)
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
	env = append(env, fmt.Sprintf("ERU_STORAGE=%d", opts.Storage))
	config.Env = env
	// basic labels, bind to LabelMeta
	config.Labels = map[string]string{
		cluster.ERUMark: "1",
		cluster.LabelMeta: utils.EncodeMetaInLabel(&types.LabelMeta{
			Publish:     opts.Entrypoint.Publish,
			HealthCheck: entry.HealthCheck,
		}),
	}
	for key, value := range opts.Labels {
		config.Labels[key] = value
	}

	return config
}
