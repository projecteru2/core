package calcium

import (
	"context"
	"fmt"
	"sync"

	"github.com/pkg/errors"
	"github.com/projecteru2/core/cluster"
	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/metrics"
	"github.com/projecteru2/core/resources"

	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
	"github.com/sanity-io/litter"
	log "github.com/sirupsen/logrus"
)

// CreateContainer use options to create containers
func (c *Calcium) CreateContainer(ctx context.Context, opts *types.DeployOptions) (chan *types.CreateContainerMessage, error) {
	// TODO: new way to normalize
	//opts.Normalize()
	opts.ProcessIdent = utils.RandomString(16)
	log.Infof("[CreateContainer %s] Creating container with options:", opts.ProcessIdent)
	litter.Dump(opts)
	// Count 要大于0
	if opts.Count <= 0 {
		return nil, errors.WithStack(types.NewDetailedErr(types.ErrBadCount, opts.Count))
	}

	ch, err := c.doCreateWorkloads(ctx, opts)
	return ch, errors.WithStack(err)
}

// transaction: resource metadata consistency
func (c *Calcium) doCreateWorkloads(ctx context.Context, opts *types.DeployOptions) (chan *types.CreateContainerMessage, error) {
	ch := make(chan *types.CreateContainerMessage)
	// RFC 计算当前 app 部署情况的时候需要保证同一时间只有这个 app 的这个 entrypoint 在跑
	// 因此需要在这里加个全局锁，直到部署完毕才释放
	// 通过 Processing 状态跟踪达成 18 Oct, 2018

	var (
		err         error
		planMap     map[types.ResourceType]resources.ResourcePlans
		deployMap   map[string]*types.DeployInfo
		rollbackMap map[string][]int
	)

	go func() {
		defer func() {
			for nodename := range deployMap {
				if e := c.store.DeleteProcessing(ctx, opts, nodename); e != nil {
					err = e
					log.Errorf("[Calcium.doCreateWorkloads] delete processing failed for %s: %+v", nodename, err)
				}
			}
			close(ch)
		}()

		if err := utils.Txn(
			ctx,

			// if: alloc resources
			func(ctx context.Context) error {
				return c.withNodesLocked(ctx, opts.Podname, opts.Nodenames, opts.NodeLabels, false, func(nodeMap map[string]*types.Node) (err error) {
					defer func() {
						if err != nil {
							ch <- &types.CreateContainerMessage{Error: err}
						}
					}()

					// calculate plans
					if planMap, deployMap, err = c.doAllocResource(ctx, nodeMap, opts); err != nil {
						return errors.WithStack(err)
					}

					// commit changes
					nodes := []*types.Node{}
					for nodeName, deployInfo := range deployMap {
						for _, plan := range planMap {
							plan.ApplyChangesOnNode(nodeMap[nodeName], utils.Range(deployInfo.Deploy)...)
						}
						nodes = append(nodes, nodeMap[nodeName])
					}
					for nodename, deployInfo := range deployMap {
						if err = c.store.SaveProcessing(ctx, opts, nodename, deployInfo.Deploy); err != nil {
							return errors.WithStack(err)
						}
					}
					return errors.WithStack(c.store.UpdateNodes(ctx, nodes...))
				})
			},

			// then: deploy containers
			func(ctx context.Context) error {
				rollbackMap, err = c.doDeployWorkloads(ctx, ch, opts, planMap, deployMap)
				return errors.WithStack(err)
			},

			// rollback: give back resources
			func(ctx context.Context) (err error) {
				for nodeName, rollbackIndices := range rollbackMap {
					indices := rollbackIndices
					if e := c.withNodeLocked(ctx, nodeName, func(node *types.Node) error {
						for _, plan := range planMap {
							plan.RollbackChangesOnNode(node, indices...)
						}
						return errors.WithStack(c.store.UpdateNodes(ctx, node))
					}); e != nil {
						err = e
					}
				}
				return errors.WithStack(err)
			},

			c.config.GlobalTimeout,
		); err != nil {
			log.Errorf("[Calcium.doCreateWorkloads] %+v", err)
		}
	}()

	return ch, err
}

func (c *Calcium) doDeployWorkloads(ctx context.Context, ch chan *types.CreateContainerMessage, opts *types.DeployOptions, planMap map[types.ResourceType]resources.ResourcePlans, deployMap map[string]*types.DeployInfo) (_ map[string][]int, err error) {
	wg := sync.WaitGroup{}
	wg.Add(len(deployMap))

	seq := 0
	rollbackMap := make(map[string][]int)
	for nodename, deployInfo := range deployMap {
		go metrics.Client.SendDeployCount(deployInfo.Deploy)
		go func(nodename string, seq int) {
			defer wg.Done()
			if indices, e := c.doDeployWorkloadsOnNode(ctx, ch, nodename, opts, deployInfo, planMap, seq); e != nil {
				err = e
				rollbackMap[nodename] = indices
			}
			//process
		}(nodename, seq)
		seq += deployInfo.Deploy
	}

	wg.Wait()
	return rollbackMap, errors.WithStack(err)
}

// deploy scheduled containers on one node
func (c *Calcium) doDeployWorkloadsOnNode(ctx context.Context, ch chan *types.CreateContainerMessage, nodeName string, opts *types.DeployOptions, deployInfo *types.DeployInfo, planMap map[types.ResourceType]resources.ResourcePlans, seq int) (indices []int, err error) {
	node, err := c.doGetAndPrepareNode(ctx, nodeName, opts.Image)
	if err != nil {
		for i := 0; i < deployInfo.Deploy; i++ {
			ch <- &types.CreateContainerMessage{Error: err}
		}
		return utils.Range(deployInfo.Deploy), errors.WithStack(err)
	}

	for idx := 0; idx < deployInfo.Deploy; idx++ {
		createMsg := &types.CreateContainerMessage{
			Podname:  opts.Podname,
			Nodename: nodeName,
			Publish:  map[string][]string{},
		}

		func() {
			var e error
			defer func() {
				if e != nil {
					err = e
					createMsg.Error = e
					indices = append(indices, idx)
				}
				ch <- createMsg
			}()

			rsc := &types.Resources{}
			o := resources.DispenseOptions{
				Node:  node,
				Index: idx,
			}
			for _, plan := range planMap {
				if e = plan.Dispense(o, rsc); e != nil {
					return
				}
			}

			createMsg.Resources = *rsc
			e = c.doDeployOneWorkload(ctx, node, opts, createMsg, seq+idx, deployInfo.Deploy-1-idx)
		}()
	}

	return indices, errors.WithStack(err)
}

func (c *Calcium) doGetAndPrepareNode(ctx context.Context, nodename, image string) (*types.Node, error) {
	node, err := c.GetNode(ctx, nodename)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return node, errors.WithStack(pullImage(ctx, node, image))
}

// transaction: container metadata consistency
func (c *Calcium) doDeployOneWorkload(
	ctx context.Context,
	node *types.Node,
	opts *types.DeployOptions,
	msg *types.CreateContainerMessage,
	no, processingCount int,
) (err error) {
	config := c.doMakeContainerOptions(no, msg, opts, node)
	container := &types.Container{
		Name:              config.Name,
		Labels:            config.Labels,
		Podname:           opts.Podname,
		Nodename:          node.Name,
		CPURequest:        msg.CPURequest,
		QuotaRequest:      msg.CPUQuotaRequest,
		QuotaLimit:        msg.CPUQuotaLimit,
		MemoryRequest:     msg.MemoryRequest,
		MemoryLimit:       msg.MemoryLimit,
		StorageRequest:    msg.StorageRequest,
		StorageLimit:      msg.StorageLimit,
		VolumeRequest:     msg.VolumeRequest,
		VolumeLimit:       msg.VolumeLimit,
		VolumePlanRequest: msg.VolumePlanRequest,
		VolumePlanLimit:   msg.VolumePlanLimit,
		Hook:              opts.Entrypoint.Hook,
		Privileged:        opts.Entrypoint.Privileged,
		Engine:            node.Engine,
		SoftLimit:         opts.SoftLimit,
		Image:             opts.Image,
		Env:               opts.Env,
		User:              opts.User,
	}
	return utils.Txn(
		ctx,
		// create container
		func(ctx context.Context) error {
			created, err := node.Engine.VirtualizationCreate(ctx, config)
			if err != nil {
				return errors.WithStack(err)
			}
			container.ID = created.ID
			return nil
		},

		func(ctx context.Context) (err error) {
			// Copy data to container
			if len(opts.Data) > 0 {
				for dst, readerManager := range opts.Data {
					reader, err := readerManager.GetReader()
					if err != nil {
						return errors.WithStack(err)
					}
					if err = c.doSendFileToContainer(ctx, node.Engine, container.ID, dst, reader, true, true); err != nil {
						return errors.WithStack(err)
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
			msg.Hook, err = c.doStartContainer(ctx, container, opts.IgnoreHook)
			if err != nil {
				return errors.WithStack(err)
			}

			// inspect real meta
			var containerInfo *enginetypes.VirtualizationInfo
			containerInfo, err = container.Inspect(ctx) // 补充静态元数据
			if err != nil {
				return errors.WithStack(err)
			}

			// update meta
			if containerInfo.Networks != nil {
				msg.Publish = utils.MakePublishInfo(containerInfo.Networks, opts.Entrypoint.Publish)
			}
			// reset users
			if containerInfo.User != container.User {
				container.User = containerInfo.User
			}
			// reset container.hook
			container.Hook = opts.Entrypoint.Hook

			// update processing
			if processingCount >= 0 {
				if err := c.store.UpdateProcessing(ctx, opts, node.Name, processingCount); err != nil {
					return errors.WithStack(err)
				}
			}

			if err := c.store.AddContainer(ctx, container); err != nil {
				return errors.WithStack(err)
			}
			msg.ContainerID = container.ID
			return nil
		},

		// remove container
		func(ctx context.Context) error {
			return errors.WithStack(c.doRemoveContainer(ctx, container, true))
		},
		c.config.GlobalTimeout,
	)
}

func (c *Calcium) doMakeContainerOptions(no int, msg *types.CreateContainerMessage, opts *types.DeployOptions, node *types.Node) *enginetypes.VirtualizationCreateOptions {
	config := &enginetypes.VirtualizationCreateOptions{}
	// general
	config.CPU = msg.CPULimit
	config.Quota = msg.CPUQuotaLimit
	config.Memory = msg.MemoryLimit
	config.Storage = msg.StorageLimit
	config.NUMANode = node.GetNUMANode(msg.CPULimit)
	config.SoftLimit = opts.SoftLimit
	config.RawArgs = opts.RawArgs
	config.Lambda = opts.Lambda
	config.User = opts.User
	config.DNS = opts.DNS
	config.Image = opts.Image
	config.Stdin = opts.OpenStdin
	config.Hosts = opts.ExtraHosts
	config.Volumes = msg.VolumeLimit.ApplyPlan(msg.VolumePlanLimit).ToStringSlice(false, true)
	config.VolumePlan = msg.VolumePlanLimit.ToLiteral()
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
	msg.ContainerName = config.Name
	// command and user
	// extra args is dynamically
	slices := utils.MakeCommandLineArgs(fmt.Sprintf("%s %s", entry.Command, opts.ExtraArgs))
	config.Cmd = slices
	// env
	env := append(opts.Env, fmt.Sprintf("APP_NAME=%s", opts.Name))
	env = append(env, fmt.Sprintf("ERU_POD=%s", opts.Podname))
	env = append(env, fmt.Sprintf("ERU_NODE_NAME=%s", node.Name))
	env = append(env, fmt.Sprintf("ERU_CONTAINER_NO=%d", no))
	env = append(env, fmt.Sprintf("ERU_MEMORY=%d", msg.MemoryLimit))
	env = append(env, fmt.Sprintf("ERU_STORAGE=%d", msg.StorageLimit))
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
