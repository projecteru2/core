package calcium

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/projecteru2/core/cluster"
	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/metrics"
	resourcetypes "github.com/projecteru2/core/resources/types"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
	"github.com/sanity-io/litter"
)

// CreateWorkload use options to create workloads
func (c *Calcium) CreateWorkload(ctx context.Context, opts *types.DeployOptions) (chan *types.CreateWorkloadMessage, error) {
	if err := opts.Validate(); err != nil {
		return nil, err
	}

	opts.ProcessIdent = utils.RandomString(16)
	log.Infof("[CreateWorkload %s] Creating workload with options:", opts.ProcessIdent)
	litter.Dump(opts)
	// Count 要大于0
	if opts.Count <= 0 {
		return nil, errors.WithStack(types.NewDetailedErr(types.ErrBadCount, opts.Count))
	}

	ch, err := c.doCreateWorkloads(ctx, opts)
	return ch, errors.WithStack(err)
}

// transaction: resource metadata consistency
func (c *Calcium) doCreateWorkloads(ctx context.Context, opts *types.DeployOptions) (chan *types.CreateWorkloadMessage, error) {
	ch := make(chan *types.CreateWorkloadMessage)
	// RFC 计算当前 app 部署情况的时候需要保证同一时间只有这个 app 的这个 entrypoint 在跑
	// 因此需要在这里加个全局锁，直到部署完毕才释放
	// 通过 Processing 状态跟踪达成 18 Oct, 2018

	var (
		err         error
		plans       []resourcetypes.ResourcePlans
		deployMap   map[string]int
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
				return c.withNodesLocked(ctx, opts.Podname, opts.Nodenames, opts.NodeLabels, false, func(ctx context.Context, nodeMap map[string]*types.Node) (err error) {
					defer func() {
						if err != nil {
							ch <- &types.CreateWorkloadMessage{Error: err}
						}
					}()

					// calculate plans
					if plans, deployMap, err = c.doAllocResource(ctx, nodeMap, opts); err != nil {
						return errors.WithStack(err)
					}

					// commit changes
					nodes := []*types.Node{}
					for nodename, deploy := range deployMap {
						for _, plan := range plans {
							plan.ApplyChangesOnNode(nodeMap[nodename], utils.Range(deploy)...)
						}
						nodes = append(nodes, nodeMap[nodename])
						if err = c.store.SaveProcessing(ctx, opts, nodename, deploy); err != nil {
							return errors.WithStack(err)
						}
					}
					return errors.WithStack(c.store.UpdateNodes(ctx, nodes...))
				})
			},

			// then: deploy workloads
			func(ctx context.Context) error {
				rollbackMap, err = c.doDeployWorkloads(ctx, ch, opts, plans, deployMap)
				return errors.WithStack(err)
			},

			// rollback: give back resources
			func(ctx context.Context, _ bool) (err error) {
				for nodename, rollbackIndices := range rollbackMap {
					if e := c.withNodeLocked(ctx, nodename, func(ctx context.Context, node *types.Node) error {
						for _, plan := range plans {
							plan.RollbackChangesOnNode(node, rollbackIndices...) // nolint:scopelint
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

	return ch, errors.WithStack(err)
}

func (c *Calcium) doDeployWorkloads(ctx context.Context, ch chan *types.CreateWorkloadMessage, opts *types.DeployOptions, plans []resourcetypes.ResourcePlans, deployMap map[string]int) (_ map[string][]int, err error) {
	wg := sync.WaitGroup{}
	wg.Add(len(deployMap))

	seq := 0
	rollbackMap := make(map[string][]int)
	for nodename, deploy := range deployMap {
		go metrics.Client.SendDeployCount(deploy)
		go func(nodename string, deploy, seq int) {
			defer wg.Done()
			if indices, e := c.doDeployWorkloadsOnNode(ctx, ch, nodename, opts, deploy, plans, seq); e != nil {
				err = e
				rollbackMap[nodename] = indices
			}
		}(nodename, deploy, seq)
		seq += deploy
	}

	wg.Wait()
	return rollbackMap, errors.WithStack(err)
}

// deploy scheduled workloads on one node
func (c *Calcium) doDeployWorkloadsOnNode(ctx context.Context, ch chan *types.CreateWorkloadMessage, nodename string, opts *types.DeployOptions, deploy int, plans []resourcetypes.ResourcePlans, seq int) (indices []int, err error) {
	node, err := c.doGetAndPrepareNode(ctx, nodename, opts.Image)
	if err != nil {
		for i := 0; i < deploy; i++ {
			ch <- &types.CreateWorkloadMessage{Error: err}
		}
		return utils.Range(deploy), errors.WithStack(err)
	}

	for idx := 0; idx < deploy; idx++ {
		createMsg := &types.CreateWorkloadMessage{
			Podname:  opts.Podname,
			Nodename: nodename,
			Publish:  map[string][]string{},
		}

		do := func(idx int) (e error) {
			defer func() {
				if e != nil {
					err = e
					createMsg.Error = e
					indices = append(indices, idx)
				}
				ch <- createMsg
			}()

			r := &types.ResourceMeta{}
			o := resourcetypes.DispenseOptions{
				Node:  node,
				Index: idx,
			}
			for _, plan := range plans {
				if r, e = plan.Dispense(o, r); e != nil {
					return
				}
			}

			createMsg.ResourceMeta = *r
			createOpts := c.doMakeWorkloadOptions(seq+idx, createMsg, opts, node)
			return c.doDeployOneWorkload(ctx, node, opts, createMsg, createOpts, deploy-1-idx)
		}
		_ = do(idx)
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

// transaction: workload metadata consistency
func (c *Calcium) doDeployOneWorkload(
	ctx context.Context,
	node *types.Node,
	opts *types.DeployOptions,
	msg *types.CreateWorkloadMessage,
	config *enginetypes.VirtualizationCreateOptions,
	processingCount int,
) (err error) {
	workload := &types.Workload{
		ResourceMeta: types.ResourceMeta{
			CPU:               msg.CPU,
			CPUQuotaRequest:   msg.CPUQuotaRequest,
			CPUQuotaLimit:     msg.CPUQuotaLimit,
			MemoryRequest:     msg.MemoryRequest,
			MemoryLimit:       msg.MemoryLimit,
			StorageRequest:    msg.StorageRequest,
			StorageLimit:      msg.StorageLimit,
			VolumeRequest:     msg.VolumeRequest,
			VolumeLimit:       msg.VolumeLimit,
			VolumePlanRequest: msg.VolumePlanRequest,
			VolumePlanLimit:   msg.VolumePlanLimit,
		},
		Name:       config.Name,
		Labels:     config.Labels,
		Podname:    opts.Podname,
		Nodename:   node.Name,
		Hook:       opts.Entrypoint.Hook,
		Privileged: opts.Entrypoint.Privileged,
		Engine:     node.Engine,
		Image:      opts.Image,
		Env:        opts.Env,
		User:       opts.User,
		CreateTime: time.Now().Unix(),
	}
	return utils.Txn(
		ctx,
		// create workload
		func(ctx context.Context) error {
			created, err := node.Engine.VirtualizationCreate(ctx, config)
			if err != nil {
				return errors.WithStack(err)
			}
			workload.ID = created.ID
			return nil
		},

		func(ctx context.Context) (err error) {
			// Copy data to workload
			if len(opts.Data) > 0 {
				for dst, readerManager := range opts.Data {
					reader, err := readerManager.GetReader()
					if err != nil {
						return errors.WithStack(err)
					}
					if err = c.doSendFileToWorkload(ctx, node.Engine, workload.ID, dst, reader, true, false); err != nil {
						return errors.WithStack(err)
					}
				}
			}

			// deal with hook
			if len(opts.AfterCreate) > 0 && workload.Hook != nil {
				workload.Hook = &types.Hook{
					AfterStart: append(opts.AfterCreate, workload.Hook.AfterStart...),
					Force:      workload.Hook.Force,
				}
			}

			// start first
			msg.Hook, err = c.doStartWorkload(ctx, workload, opts.IgnoreHook)
			if err != nil {
				return errors.WithStack(err)
			}

			// inspect real meta
			var workloadInfo *enginetypes.VirtualizationInfo
			workloadInfo, err = workload.Inspect(ctx) // 补充静态元数据
			if err != nil {
				return errors.WithStack(err)
			}

			// update meta
			if workloadInfo.Networks != nil {
				msg.Publish = utils.MakePublishInfo(workloadInfo.Networks, opts.Entrypoint.Publish)
			}
			// reset users
			if workloadInfo.User != workload.User {
				workload.User = workloadInfo.User
			}
			// reset workload.hook
			workload.Hook = opts.Entrypoint.Hook

			// update processing
			if processingCount >= 0 {
				if err := c.store.UpdateProcessing(ctx, opts, node.Name, processingCount); err != nil {
					return errors.WithStack(err)
				}
			}

			if err := c.store.AddWorkload(ctx, workload); err != nil {
				return errors.WithStack(err)
			}
			msg.WorkloadID = workload.ID
			return nil
		},

		// remove workload
		func(ctx context.Context, _ bool) error {
			return errors.WithStack(c.doRemoveWorkload(ctx, workload, true))
		},
		c.config.GlobalTimeout,
	)
}

func (c *Calcium) doMakeWorkloadOptions(no int, msg *types.CreateWorkloadMessage, opts *types.DeployOptions, node *types.Node) *enginetypes.VirtualizationCreateOptions {
	config := &enginetypes.VirtualizationCreateOptions{}
	// general
	config.CPU = msg.CPU
	config.Quota = msg.CPUQuotaLimit
	config.Memory = msg.MemoryLimit
	config.Storage = msg.StorageLimit
	config.NUMANode = msg.NUMANode
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
	config.Name = utils.MakeWorkloadName(opts.Name, opts.Entrypoint.Name, suffix)
	msg.WorkloadName = config.Name
	// command and user
	// extra args is dynamically
	slices := utils.MakeCommandLineArgs(fmt.Sprintf("%s %s", entry.Command, opts.ExtraArgs))
	config.Cmd = slices
	// env
	env := append(opts.Env, fmt.Sprintf("APP_NAME=%s", opts.Name))
	env = append(env, fmt.Sprintf("ERU_POD=%s", opts.Podname))
	env = append(env, fmt.Sprintf("ERU_NODE_NAME=%s", node.Name))
	env = append(env, fmt.Sprintf("ERU_WORKLOAD_SEQ=%d", no))
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
