package calcium

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/sanity-io/litter"

	"github.com/projecteru2/core/cluster"
	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/metrics"
	resourcetypes "github.com/projecteru2/core/resources/types"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
	"github.com/projecteru2/core/wal"
)

// CreateWorkload use options to create workloads
func (c *Calcium) CreateWorkload(ctx context.Context, opts *types.DeployOptions) (chan *types.CreateWorkloadMessage, error) {
	logger := log.WithField("Calcium", "CreateWorkload").WithField("opts", opts)
	if err := opts.Validate(); err != nil {
		return nil, logger.Err(err)
	}

	opts.ProcessIdent = utils.RandomString(16)
	log.Infof("[CreateWorkload %s] Creating workload with options:", opts.ProcessIdent)
	litter.Dump(opts)
	// Count 要大于0
	if opts.Count <= 0 {
		return nil, logger.Err(errors.WithStack(types.NewDetailedErr(types.ErrBadCount, opts.Count)))
	}

	return c.doCreateWorkloads(ctx, opts), nil
}

// getDeployNodenames checks nodenames and excludedNodenames
// if excludedNodenames is not set or nodenames is set, then return nodenames
// for other cases, then node within excludedNodename list will not be returned
func (c *Calcium) getDeployNodenames(ctx context.Context, podname string, nodenames, excludedNodenames []string) ([]string, error) {
	if len(excludedNodenames) == 0 || len(nodenames) != 0 {
		return nodenames, nil
	}

	allNodes, err := c.ListPodNodes(ctx, podname, map[string]string{}, false)
	if err != nil {
		return nil, err
	}

	excludes := map[string]struct{}{}
	for _, n := range excludedNodenames {
		excludes[n] = struct{}{}
	}

	rv := []string{}
	for _, n := range allNodes {
		if _, ok := excludes[n.Name]; ok {
			continue
		}
		rv = append(rv, n.Name)
	}
	return rv, nil
}

// transaction: resource metadata consistency
func (c *Calcium) doCreateWorkloads(ctx context.Context, opts *types.DeployOptions) chan *types.CreateWorkloadMessage {
	logger := log.WithField("Calcium", "doCreateWorkloads").WithField("opts", opts)
	ch := make(chan *types.CreateWorkloadMessage)
	// RFC 计算当前 app 部署情况的时候需要保证同一时间只有这个 app 的这个 entrypoint 在跑
	// 因此需要在这里加个全局锁，直到部署完毕才释放
	// 通过 Processing 状态跟踪达成 18 Oct, 2018

	var (
		plans       []resourcetypes.ResourcePlans
		deployMap   map[string]int
		rollbackMap map[string][]int
	)

	go func() {
		defer func() {
			for nodename := range deployMap {
				if e := c.store.DeleteProcessing(context.Background(), opts, nodename); e != nil {
					logger.Errorf("[Calcium.doCreateWorkloads] delete processing failed for %s: %+v", nodename, e)
				}
			}
			close(ch)
		}()

		nodenames, err := c.getDeployNodenames(ctx, opts.Podname, opts.Nodenames, opts.ExcludedNodenames)
		if err != nil {
			ch <- &types.CreateWorkloadMessage{Error: logger.Err(err)}
			return
		}

		_ = utils.Txn(
			ctx,

			// if: alloc resources
			func(ctx context.Context) error {
				return c.withNodesLocked(ctx, opts.Podname, nodenames, opts.NodeLabels, false, func(ctx context.Context, nodeMap map[string]*types.Node) (err error) {
					defer func() {
						if err != nil {
							ch <- &types.CreateWorkloadMessage{Error: logger.Err(err)}
						}
					}()

					// calculate plans
					if plans, deployMap, err = c.doAllocResource(ctx, nodeMap, opts); err != nil {
						return err
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
			func(ctx context.Context) (err error) {
				rollbackMap, err = c.doDeployWorkloads(ctx, ch, opts, plans, deployMap)
				return err
			},

			// rollback: give back resources
			func(ctx context.Context, failedOnCond bool) (err error) {
				if failedOnCond {
					return
				}
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
				return err
			},

			c.config.GlobalTimeout,
		)
	}()

	return ch
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
	log.Debugf("[Calcium.doDeployWorkloads] rollbackMap: %+v", rollbackMap)
	return rollbackMap, err
}

// deploy scheduled workloads on one node
func (c *Calcium) doDeployWorkloadsOnNode(ctx context.Context, ch chan *types.CreateWorkloadMessage, nodename string, opts *types.DeployOptions, deploy int, plans []resourcetypes.ResourcePlans, seq int) (indices []int, err error) {
	logger := log.WithField("Calcium", "doDeployWorkloadsOnNode").WithField("nodename", nodename).WithField("opts", opts).WithField("deploy", deploy).WithField("plans", plans).WithField("seq", seq)
	node, err := c.doGetAndPrepareNode(ctx, nodename, opts.Image)
	if err != nil {
		for i := 0; i < deploy; i++ {
			ch <- &types.CreateWorkloadMessage{Error: logger.Err(err)}
		}
		return utils.Range(deploy), err
	}

	pool, appendLock := utils.NewGoroutinePool(int(c.config.MaxConcurrency)), sync.Mutex{}
	for idx := 0; idx < deploy; idx++ {
		createMsg := &types.CreateWorkloadMessage{
			Podname:  opts.Podname,
			Nodename: nodename,
			Publish:  map[string][]string{},
		}

		pool.Go(func(idx int) func() {
			return func() {
				var e error
				defer func() {
					if e != nil {
						err = e
						createMsg.Error = logger.Err(e)
						appendLock.Lock()
						indices = append(indices, idx)
						appendLock.Unlock()
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
				e = c.doDeployOneWorkload(ctx, node, opts, createMsg, createOpts, deploy-1-idx)
			}
		}(idx))
	}
	pool.Wait()

	// remap 就不搞进事务了吧, 回滚代价太大了
	// 放任 remap 失败的后果是, share pool 没有更新, 这个后果姑且认为是可以承受的
	// 而且 remap 是一个幂等操作, 就算这次 remap 失败, 下次 remap 也能收敛到正确到状态
	if err := c.withNodeLocked(context.Background(), nodename, func(ctx context.Context, node *types.Node) error {
		c.doRemapResourceAndLog(context.Background(), logger, node)
		return nil
	}); err != nil {
		logger.Errorf("failed to lock node to remap: %v", err)
	}
	return indices, err
}

func (c *Calcium) doGetAndPrepareNode(ctx context.Context, nodename, image string) (*types.Node, error) {
	node, err := c.GetNode(ctx, nodename)
	if err != nil {
		return nil, err
	}

	return node, pullImage(ctx, node, image)
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
	var commit wal.Commit
	defer func() {
		if commit != nil {
			if err := commit(context.Background()); err != nil {
				log.Errorf("[doDeployOneWorkload] Commit WAL %s failed: %v", eventCreateWorkload, err)
			}
		}
	}()
	return utils.Txn(
		ctx,
		// create workload
		func(ctx context.Context) error {
			created, err := node.Engine.VirtualizationCreate(ctx, config)
			if err != nil {
				return errors.WithStack(err)
			}
			workload.ID = created.ID
			// We couldn't WAL the workload ID above VirtualizationCreate temporarily,
			// so there's a time gap window, once the core process crashes between
			// VirtualizationCreate and logCreateWorkload then the worload is leaky.
			if commit, err = c.wal.logCreateWorkload(ctx, workload.ID, node.Name); err != nil {
				return err
			}
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
						return err
					}
				}
			}

			// deal with hook
			if len(opts.AfterCreate) > 0 {
				if workload.Hook != nil {
					workload.Hook = &types.Hook{
						AfterStart: append(opts.AfterCreate, workload.Hook.AfterStart...),
						Force:      workload.Hook.Force,
					}
				} else {
					workload.Hook = &types.Hook{
						AfterStart: opts.AfterCreate,
						Force:      opts.IgnoreHook,
					}
				}
			}

			// start first
			msg.Hook, err = c.doStartWorkload(ctx, workload, opts.IgnoreHook)
			if err != nil {
				return err
			}

			// inspect real meta
			var workloadInfo *enginetypes.VirtualizationInfo
			workloadInfo, err = workload.Inspect(ctx) // 补充静态元数据
			if err != nil {
				return err
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
			msg.WorkloadName = workload.Name
			msg.Podname = workload.Podname
			msg.Nodename = workload.Nodename
			return nil
		},

		// remove workload
		func(ctx context.Context, _ bool) error {
			if workload.ID == "" {
				return nil
			}
			return c.doRemoveWorkload(ctx, workload, true)
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
	config.Sysctl = entry.Sysctls
	config.Publish = entry.Publish
	config.Restart = entry.Restart
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
	if msg.CPU != nil {
		bs, _ := json.Marshal(msg.CPU)
		env = append(env, fmt.Sprintf("ERU_CPU=%s", bs))
	}
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
