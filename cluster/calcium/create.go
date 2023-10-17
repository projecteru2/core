package calcium

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/alphadose/haxmap"
	"github.com/cockroachdb/errors"

	"github.com/projecteru2/core/cluster"
	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/metrics"
	resourcetypes "github.com/projecteru2/core/resource/types"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
	"github.com/projecteru2/core/wal"

	"github.com/sanity-io/litter"
)

// CreateWorkload use options to create workloads
func (c *Calcium) CreateWorkload(ctx context.Context, opts *types.DeployOptions) (chan *types.CreateWorkloadMessage, error) {
	logger := log.WithFunc("calcium.CreateWorkload").WithField("opts", opts)
	if err := opts.Validate(); err != nil {
		logger.Error(ctx, err)
		return nil, err
	}
	opts.ProcessIdent = utils.RandomString(16)
	logger = logger.WithField("ident", opts.ProcessIdent)
	logger.Infof(ctx, "Creating workload ident %s with options:\n%s", opts.ProcessIdent, litter.Options{Compact: true}.Sdump(opts))
	// Count 要大于0
	if opts.Count <= 0 {
		err := errors.Wrapf(types.ErrInvaildDeployCount, "count: %d", opts.Count)
		logger.Error(ctx, err)
		return nil, err
	}

	return c.doCreateWorkloads(ctx, opts), nil
}

// transaction: resource metadata consistency
func (c *Calcium) doCreateWorkloads(ctx context.Context, opts *types.DeployOptions) chan *types.CreateWorkloadMessage {
	logger := log.WithFunc("calcium.doCreateWorkloads").WithField("ident", opts.ProcessIdent)
	ch := make(chan *types.CreateWorkloadMessage)
	// RFC 计算当前 app 部署情况的时候需要保证同一时间只有这个 app 的这个 entrypoint 在跑
	// 因此需要在这里加个全局锁，直到部署完毕才释放
	// 通过 Processing 状态跟踪达成 18 Oct, 2018

	var (
		deployMap   map[string]int
		rollbackMap map[string][]int
		// map[nodename][]Resources
		engineParamsMap = map[string][]resourcetypes.Resources{}
		// map[nodename][]Resources
		workloadResourcesMap = map[string][]resourcetypes.Resources{}
	)

	_ = c.pool.Invoke(func() {
		defer func() {
			cctx, cancel := context.WithTimeout(utils.NewInheritCtx(ctx), c.config.GlobalTimeout)
			for nodename := range deployMap {
				processing := opts.GetProcessing(nodename)
				if err := c.store.DeleteProcessing(cctx, processing); err != nil {
					logger.Errorf(ctx, err, "delete processing failed for %s", nodename)
				}
			}
			close(ch)
			cancel()
		}()

		var resourceCommit wal.Commit
		defer func() {
			if resourceCommit != nil {
				if err := resourceCommit(); err != nil {
					logger.Errorf(ctx, err, "commit wal failed: %s", eventWorkloadResourceAllocated)
				}
			}
		}()

		var processingCommits map[string]wal.Commit
		defer func() {
			for nodename := range processingCommits {
				if commit, ok := processingCommits[nodename]; ok {
					if err := commit(); err != nil {
						logger.Errorf(ctx, err, "commit wal failed: %s, %s", eventProcessingCreated, nodename)
					}
				}
			}
		}()

		_ = utils.Txn(
			ctx,

			// if: alloc resources
			func(ctx context.Context) (err error) {
				defer func() {
					if err != nil {
						logger.Error(ctx, err)
						ch <- &types.CreateWorkloadMessage{Error: err}
					}
				}()
				return c.withNodesPodLocked(ctx, opts.NodeFilter, func(ctx context.Context, nodeMap map[string]*types.Node) (err error) {
					if len(nodeMap) == 0 {
						return types.ErrEmptyNodeMap
					}
					nodenames := []string{}
					nodes := []*types.Node{}
					for nodename, node := range nodeMap {
						nodenames = append(nodenames, nodename)
						nodes = append(nodes, node)
					}

					if resourceCommit, err = c.wal.Log(eventWorkloadResourceAllocated, nodes); err != nil {
						return err
					}

					deployMap, err = c.doGetDeployStrategy(ctx, nodenames, opts)
					if err != nil {
						return err
					}

					// commit changes
					processingCommits = make(map[string]wal.Commit)
					for nodename, deploy := range deployMap {
						nodes = append(nodes, nodeMap[nodename])
						if workloadResourcesMap[nodename], engineParamsMap[nodename], err = c.rmgr.Alloc(ctx, nodename, deploy, opts.Resources); err != nil {
							return err
						}
						processing := opts.GetProcessing(nodename)
						if processingCommits[nodename], err = c.wal.Log(eventProcessingCreated, processing); err != nil {
							return err
						}
						if err = c.store.CreateProcessing(ctx, processing, deploy); err != nil {
							return err
						}
					}
					return nil
				})
			},

			// then: deploy workloads
			func(ctx context.Context) (err error) {
				rollbackMap, err = c.doDeployWorkloads(ctx, ch, opts, engineParamsMap, workloadResourcesMap, deployMap)
				return err
			},

			// rollback: give back resources
			func(ctx context.Context, failedOnCond bool) (err error) {
				if failedOnCond {
					return
				}
				for nodename, rollbackIndices := range rollbackMap {
					if e := c.withNodePodLocked(ctx, nodename, func(ctx context.Context, node *types.Node) error {
						rollbackResources := utils.Map(rollbackIndices, func(idx int) resourcetypes.Resources {
							return workloadResourcesMap[nodename][idx]
						})
						return c.rmgr.RollbackAlloc(ctx, nodename, rollbackResources)
					}); e != nil {
						logger.Error(ctx, e)
						err = e
					}
				}
				return err
			},

			c.config.GlobalTimeout,
		)
	})

	return ch
}

func (c *Calcium) doDeployWorkloads(ctx context.Context,
	ch chan *types.CreateWorkloadMessage,
	opts *types.DeployOptions,
	engineParamsMap map[string][]resourcetypes.Resources,
	workloadResourcesMap map[string][]resourcetypes.Resources,
	deployMap map[string]int) (_ map[string][]int, err error) {

	wg := sync.WaitGroup{}
	wg.Add(len(deployMap))
	syncRollbackMap := haxmap.New[string, []int]()
	logger := log.WithFunc("calcium.doDeployWorkloads").WithField("ident", opts.ProcessIdent)

	seq := 0
	rollbackMap := make(map[string][]int)
	for nodename, deploy := range deployMap {
		_ = c.pool.Invoke(func(deploy int) func() {
			return func() {
				metrics.Client.SendDeployCount(ctx, deploy)
			}
		}(deploy))
		_ = c.pool.Invoke(func(nodename string, deploy, seq int) func() {
			return func() {
				defer wg.Done()
				if indices, err := c.doDeployWorkloadsOnNode(ctx, ch, nodename, opts, deploy, engineParamsMap[nodename], workloadResourcesMap[nodename], seq); err != nil {
					syncRollbackMap.Set(nodename, indices)
				}
			}
		}(nodename, deploy, seq))

		seq += deploy
	}

	wg.Wait()
	syncRollbackMap.ForEach(func(nodename string, indices []int) bool {
		rollbackMap[nodename] = indices
		return true
	})
	logger.Debugf(ctx, "rollbackMap: %+v", rollbackMap)
	if len(rollbackMap) != 0 {
		err = types.ErrRollbackMapIsNotEmpty
	}
	return rollbackMap, err
}

// deploy scheduled workloads on one node
func (c *Calcium) doDeployWorkloadsOnNode(ctx context.Context,
	ch chan *types.CreateWorkloadMessage,
	nodename string,
	opts *types.DeployOptions,
	deploy int,
	engineParams []resourcetypes.Resources,
	workloadResources []resourcetypes.Resources,
	seq int) (indices []int, err error) {

	logger := log.WithFunc("calcium.doDeployWorkloadsOnNode").WithField("node", nodename).WithField("ident", opts.ProcessIdent).WithField("deploy", deploy).WithField("seq", seq)
	node, err := c.doGetAndPrepareNode(ctx, nodename, opts.Image)
	if err != nil {
		for i := 0; i < deploy; i++ {
			logger.Error(ctx, err)
			ch <- &types.CreateWorkloadMessage{Error: err}
		}
		return utils.Range(deploy), err
	}

	appendLock := sync.Mutex{}
	wg := &sync.WaitGroup{}
	wg.Add(deploy)
	for idx := 0; idx < deploy; idx++ {
		idx := idx
		createMsg := &types.CreateWorkloadMessage{
			Podname:  opts.Podname,
			Nodename: nodename,
			Publish:  map[string][]string{},
		}

		_ = c.pool.Invoke(func() {
			defer wg.Done()
			var e error
			defer func() {
				if e != nil {
					err = e
					logger.Error(ctx, err)
					createMsg.Error = err
					appendLock.Lock()
					indices = append(indices, idx)
					appendLock.Unlock()
				}
				ch <- createMsg
			}()

			createMsg.EngineParams = engineParams[idx]
			createMsg.Resources = workloadResources[idx]

			createOpts := c.doMakeWorkloadOptions(ctx, seq+idx, createMsg, opts, node)
			e = c.doDeployOneWorkload(ctx, node, opts, createMsg, createOpts, true)
		})
	}
	wg.Wait()

	// remap 就不搞进事务了吧, 回滚代价太大了
	// 放任 remap 失败的后果是, share pool 没有更新, 这个后果姑且认为是可以承受的
	// 而且 remap 是一个幂等操作, 就算这次 remap 失败, 下次 remap 也能收敛到正确到状态
	_ = c.pool.Invoke(func() { c.RemapResourceAndLog(ctx, logger, node) })

	return indices, err
}

func (c *Calcium) doGetAndPrepareNode(ctx context.Context, nodename, image string) (*types.Node, error) {
	node, err := c.store.GetNode(ctx, nodename)
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
	createOpts *enginetypes.VirtualizationCreateOptions,
	decrProcessing bool,
) (err error) {
	logger := log.WithFunc("calcium.doDeployWorkload").WithField("node", node.Name).WithField("ident", opts.ProcessIdent).WithField("msg", msg)
	workload := &types.Workload{
		Resources:    msg.Resources,
		EngineParams: msg.EngineParams,
		Name:         createOpts.Name,
		Labels:       createOpts.Labels,
		Podname:      opts.Podname,
		Nodename:     node.Name,
		Hook:         opts.Entrypoint.Hook,
		Privileged:   opts.Entrypoint.Privileged,
		Engine:       node.Engine,
		Image:        opts.Image,
		Env:          opts.Env,
		User:         opts.User,
		CreateTime:   time.Now().Unix(),
	}

	var commit wal.Commit
	defer func() {
		if commit != nil {
			if err := commit(); err != nil {
				logger.Errorf(ctx, err, "Commit WAL %s failed", eventWorkloadCreated)
			}
		}
	}()
	return utils.Txn(
		ctx,
		// create workload
		func(ctx context.Context) error {
			created, err := node.Engine.VirtualizationCreate(ctx, createOpts)
			if err != nil {
				return err
			}
			workload.ID = created.ID

			for key, value := range created.Labels { // add Labels
				workload.Labels[key] = value
			}

			// We couldn't WAL the workload ID above VirtualizationCreate temporarily,
			// so there's a time gap window, once the core process crashes between
			// VirtualizationCreate and logCreateWorkload then the worload is leaky.
			commit, err = c.wal.Log(eventWorkloadCreated, &types.Workload{
				ID:       workload.ID,
				Nodename: workload.Nodename,
			})
			return err
		},

		func(ctx context.Context) (err error) {
			// avoid to be interrupted by MakeDeployStatus
			processing := opts.GetProcessing(node.Name)
			if !decrProcessing {
				processing = nil
			}
			// add workload metadata first
			if err := c.store.AddWorkload(ctx, workload, processing); err != nil {
				return err
			}
			logger.Infof(ctx, "workload %s metadata created", workload.ID)

			// Copy data to workload
			if len(opts.Files) > 0 {
				for _, file := range opts.Files {
					if err = c.doSendFileToWorkload(ctx, node.Engine, workload.ID, file); err != nil {
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

			// start workload
			msg.Hook, err = c.doStartWorkload(ctx, workload, opts.IgnoreHook)
			if err != nil {
				return err
			}

			// reset workload.hook
			workload.Hook = opts.Entrypoint.Hook

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

			// if workload metadata changed, then update
			if workloadInfo.User != workload.User {
				// reset users
				workload.User = workloadInfo.User

				if err := c.store.UpdateWorkload(ctx, workload); err != nil {
					return err
				}
				logger.Infof(ctx, "workload %s metadata updated", workload.ID)
			}

			msg.WorkloadID = workload.ID
			msg.WorkloadName = workload.Name
			msg.Podname = workload.Podname
			msg.Nodename = workload.Nodename
			return nil
		},

		// remove workload
		func(ctx context.Context, _ bool) error {
			logger.Infof(ctx, "failed to deploy workload %s, rollback", workload.ID)
			if workload.ID == "" {
				return nil
			}

			if err := c.store.RemoveWorkload(ctx, workload); err != nil {
				logger.Errorf(ctx, err, "failed to remove workload %s", workload.ID)
			}

			return workload.Remove(ctx, true)
		},
		c.config.GlobalTimeout,
	)
}

func (c *Calcium) doMakeWorkloadOptions(ctx context.Context, no int, msg *types.CreateWorkloadMessage, opts *types.DeployOptions, node *types.Node) *enginetypes.VirtualizationCreateOptions {
	createOpts := &enginetypes.VirtualizationCreateOptions{}
	// general
	createOpts.EngineParams = msg.EngineParams
	createOpts.RawArgs = opts.RawArgs
	createOpts.Lambda = opts.Lambda
	createOpts.User = opts.User
	createOpts.DNS = opts.DNS
	createOpts.Image = opts.Image
	createOpts.Stdin = opts.OpenStdin
	createOpts.Hosts = opts.ExtraHosts
	createOpts.Debug = opts.Debug
	createOpts.Networks = opts.Networks

	// entry
	entry := opts.Entrypoint
	createOpts.WorkingDir = entry.Dir
	createOpts.Privileged = entry.Privileged
	createOpts.Sysctl = entry.Sysctls
	createOpts.Publish = entry.Publish
	createOpts.Restart = entry.Restart
	if entry.Log != nil {
		createOpts.LogType = entry.Log.Type
		createOpts.LogConfig = map[string]string{}
		for k, v := range entry.Log.Config {
			createOpts.LogConfig[k] = v
		}
	}
	// name
	suffix := utils.RandomString(6)
	createOpts.Name = utils.MakeWorkloadName(opts.Name, opts.Entrypoint.Name, suffix)
	msg.WorkloadName = createOpts.Name
	// command and user
	// extra args is dynamically
	createOpts.Cmd = opts.Entrypoint.Commands
	// env
	env := append(opts.Env, fmt.Sprintf("APP_NAME=%s", opts.Name)) //nolint
	env = append(env, fmt.Sprintf("ERU_POD=%s", opts.Podname))
	env = append(env, fmt.Sprintf("ERU_NODE_NAME=%s", node.Name))
	env = append(env, fmt.Sprintf("ERU_WORKLOAD_SEQ=%d", no))
	createOpts.Env = env
	// basic labels, bind to LabelMeta
	createOpts.Labels = map[string]string{
		cluster.ERUMark: "1",
		cluster.LabelMeta: utils.EncodeMetaInLabel(ctx, &types.LabelMeta{
			Publish:     opts.Entrypoint.Publish,
			HealthCheck: entry.HealthCheck,
		}),
		cluster.LabelNodeName: node.Name,
		cluster.LabelCoreID:   c.identifier,
	}
	for key, value := range opts.Labels {
		createOpts.Labels[key] = value
	}

	return createOpts
}
