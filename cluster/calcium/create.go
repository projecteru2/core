package calcium

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/cornelk/hashmap"

	"github.com/projecteru2/core/cluster"
	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/metrics"
	"github.com/projecteru2/core/resources"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
	"github.com/projecteru2/core/wal"

	"github.com/pkg/errors"
	"github.com/sanity-io/litter"
)

// CreateWorkload use options to create workloads
func (c *Calcium) CreateWorkload(ctx context.Context, opts *types.DeployOptions) (chan *types.CreateWorkloadMessage, error) {
	logger := log.WithField("Calcium", "CreateWorkload").WithField("opts", opts)
	if err := opts.Validate(); err != nil {
		return nil, logger.Err(ctx, err)
	}

	opts.ProcessIdent = utils.RandomString(16)
	log.Infof(ctx, "[CreateWorkload %s] Creating workload with options:\n%s", opts.ProcessIdent, litter.Options{Compact: true}.Sdump(opts))
	// Count 要大于0
	if opts.Count <= 0 {
		return nil, logger.Err(ctx, errors.WithStack(types.NewDetailedErr(types.ErrBadCount, opts.Count)))
	}

	return c.doCreateWorkloads(ctx, opts), nil
}

// transaction: resource metadata consistency
func (c *Calcium) doCreateWorkloads(ctx context.Context, opts *types.DeployOptions) chan *types.CreateWorkloadMessage {
	logger := log.WithField("Calcium", "doCreateWorkloads").WithField("opts", opts)
	ch := make(chan *types.CreateWorkloadMessage)
	// RFC 计算当前 app 部署情况的时候需要保证同一时间只有这个 app 的这个 entrypoint 在跑
	// 因此需要在这里加个全局锁，直到部署完毕才释放
	// 通过 Processing 状态跟踪达成 18 Oct, 2018

	var (
		deployMap   map[string]int
		rollbackMap map[string][]int
		// map[node][]engineArgs
		engineArgsMap = map[string][]types.EngineArgs{}
		// map[node][]map[plugin]resourceArgs
		resourceArgsMap = map[string][]map[string]types.WorkloadResourceArgs{}
	)

	utils.SentryGo(func() {
		defer func() {
			cctx, cancel := context.WithTimeout(utils.InheritTracingInfo(ctx, context.TODO()), c.config.GlobalTimeout)
			for nodename := range deployMap {
				processing := opts.GetProcessing(nodename)
				if err := c.store.DeleteProcessing(cctx, processing); err != nil {
					logger.Errorf(ctx, "[Calcium.doCreateWorkloads] delete processing failed for %s: %+v", nodename, err)
				}
			}
			close(ch)
			cancel()
		}()

		var resourceCommit wal.Commit
		defer func() {
			if resourceCommit != nil {
				if err := resourceCommit(); err != nil {
					logger.Errorf(ctx, "commit wal failed: %s, %+v", eventWorkloadResourceAllocated, err)
				}
			}
		}()

		var processingCommits map[string]wal.Commit
		defer func() {
			for nodename := range processingCommits {
				if processingCommits[nodename] == nil {
					continue
				}
				if err := processingCommits[nodename](); err != nil {
					logger.Errorf(ctx, "commit wal failed: %s, %s, %+v", eventProcessingCreated, nodename, err)
				}
			}
		}()

		_ = utils.Txn(
			ctx,

			// if: alloc resources
			func(ctx context.Context) (err error) {
				defer func() {
					if err != nil {
						ch <- &types.CreateWorkloadMessage{Error: logger.Err(ctx, err)}
					}
				}()
				return c.withNodesResourceLocked(ctx, opts.NodeFilter, func(ctx context.Context, nodeMap map[string]*types.Node) (err error) {
					nodeNames := []string{}
					nodes := []*types.Node{}
					for nodeName, node := range nodeMap {
						nodeNames = append(nodeNames, nodeName)
						nodes = append(nodes, node)
					}

					if resourceCommit, err = c.wal.Log(eventWorkloadResourceAllocated, nodes); err != nil {
						return errors.WithStack(err)
					}

					deployMap, err = c.doGetDeployMap(ctx, nodeNames, opts)
					if err != nil {
						return err
					}

					// commit changes
					processingCommits = make(map[string]wal.Commit)
					for nodeName, deploy := range deployMap {
						nodes = append(nodes, nodeMap[nodeName])
						if engineArgsMap[nodeName], resourceArgsMap[nodeName], err = c.resource.Alloc(ctx, nodeName, deploy, opts.ResourceOpts); err != nil {
							return errors.WithStack(err)
						}

						processing := opts.GetProcessing(nodeName)
						if processingCommits[nodeName], err = c.wal.Log(eventProcessingCreated, processing); err != nil {
							return errors.WithStack(err)
						}
						if err = c.store.CreateProcessing(ctx, processing, deploy); err != nil {
							return errors.WithStack(err)
						}
					}
					// todo: don't have to update nodes?
					return errors.WithStack(c.store.UpdateNodes(ctx, nodes...))
				})
			},

			// then: deploy workloads
			func(ctx context.Context) (err error) {
				rollbackMap, err = c.doDeployWorkloads(ctx, ch, opts, engineArgsMap, resourceArgsMap, deployMap)
				return err
			},

			// rollback: give back resources
			func(ctx context.Context, failedOnCond bool) (err error) {
				if failedOnCond {
					return
				}
				for nodename, rollbackIndices := range rollbackMap {
					if e := c.withNodeResourceLocked(ctx, nodename, func(ctx context.Context, node *types.Node) error {
						resourceArgsToRollback := []map[string]types.WorkloadResourceArgs{}
						for _, idx := range rollbackIndices {
							resourceArgsToRollback = append(resourceArgsToRollback, resourceArgsMap[nodename][idx])
						}
						_, _, err := c.resource.SetNodeResourceUsage(ctx, nodename, nil, nil, resourceArgsToRollback, true, resources.Decr)
						return err
					}); e != nil {
						err = logger.Err(ctx, e)
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
	engineArgsMap map[string][]types.EngineArgs,
	resourceArgsMap map[string][]map[string]types.WorkloadResourceArgs,
	deployMap map[string]int) (_ map[string][]int, err error) {

	wg := sync.WaitGroup{}
	wg.Add(len(deployMap))
	syncRollbackMap := hashmap.HashMap{}

	seq := 0
	rollbackMap := make(map[string][]int)
	for nodename, deploy := range deployMap {
		utils.SentryGo(func(deploy int) func() {
			return func() {
				metrics.Client.SendDeployCount(deploy)
			}
		}(deploy))
		utils.SentryGo(func(nodename string, deploy, seq int) func() {
			return func() {
				defer wg.Done()
				if indices, err := c.doDeployWorkloadsOnNode(ctx, ch, nodename, opts, deploy, engineArgsMap[nodename], resourceArgsMap[nodename], seq); err != nil {
					syncRollbackMap.Set(nodename, indices)
				}
			}
		}(nodename, deploy, seq))

		seq += deploy
	}

	wg.Wait()
	for kv := range syncRollbackMap.Iter() {
		nodename := kv.Key.(string)
		indices := kv.Value.([]int)
		rollbackMap[nodename] = indices
	}
	log.Debugf(ctx, "[Calcium.doDeployWorkloads] rollbackMap: %+v", rollbackMap)
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
	engineArgs []types.EngineArgs,
	resourceArgs []map[string]types.WorkloadResourceArgs,
	seq int) (indices []int, err error) {

	logger := log.WithField("Calcium", "doDeployWorkloadsOnNode").WithField("nodename", nodename).WithField("opts", opts).WithField("deploy", deploy).WithField("seq", seq)
	node, err := c.doGetAndPrepareNode(ctx, nodename, opts.Image)
	if err != nil {
		for i := 0; i < deploy; i++ {
			ch <- &types.CreateWorkloadMessage{Error: logger.Err(ctx, err)}
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

		pool.Go(ctx, func(idx int) func() {
			return func() {
				var e error
				defer func() {
					if e != nil {
						err = e
						createMsg.Error = logger.Err(ctx, e)
						appendLock.Lock()
						indices = append(indices, idx)
						appendLock.Unlock()
					}
					ch <- createMsg
				}()

				createMsg.EngineArgs = engineArgs[idx]
				createMsg.ResourceArgs = map[string]types.WorkloadResourceArgs{}
				for k, v := range resourceArgs[idx] {
					createMsg.ResourceArgs[k] = v
				}

				createOpts := c.doMakeWorkloadOptions(ctx, seq+idx, createMsg, opts, node)
				e = c.doDeployOneWorkload(ctx, node, opts, createMsg, createOpts, true)
			}
		}(idx))
	}
	pool.Wait(ctx)

	// remap 就不搞进事务了吧, 回滚代价太大了
	// 放任 remap 失败的后果是, share pool 没有更新, 这个后果姑且认为是可以承受的
	// 而且 remap 是一个幂等操作, 就算这次 remap 失败, 下次 remap 也能收敛到正确到状态
	go c.doRemapResourceAndLog(ctx, logger, node)

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
	decrProcessing bool,
) (err error) {
	logger := log.WithField("Calcium", "doDeployWorkload").WithField("nodename", node.Name).WithField("opts", opts).WithField("msg", msg)
	workload := &types.Workload{
		ResourceArgs: types.ResourceMeta{},
		EngineArgs:   msg.EngineArgs,
		Name:         config.Name,
		Labels:       config.Labels,
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
	// copy resource args
	for k, v := range msg.ResourceArgs {
		workload.ResourceArgs[k] = v
	}

	var commit wal.Commit
	defer func() {
		if commit != nil {
			if err := commit(); err != nil {
				logger.Errorf(ctx, "Commit WAL %s failed: %+v", eventWorkloadCreated, err)
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
			return errors.WithStack(err)
		},

		func(ctx context.Context) (err error) {
			// avoid to be interrupted by MakeDeployStatus
			processing := opts.GetProcessing(node.Name)
			if !decrProcessing {
				processing = nil
			}
			// add workload metadata first
			if err := c.store.AddWorkload(ctx, workload, processing); err != nil {
				return errors.WithStack(err)
			}
			log.Infof(ctx, "[doDeployOneWorkload] workload %s metadata created", workload.ID)

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
					return errors.WithStack(err)
				}
				log.Infof(ctx, "[doDeployOneWorkload] workload %s metadata updated", workload.ID)
			}

			msg.WorkloadID = workload.ID
			msg.WorkloadName = workload.Name
			msg.Podname = workload.Podname
			msg.Nodename = workload.Nodename
			return nil
		},

		// remove workload
		func(ctx context.Context, _ bool) error {
			logger.Errorf(ctx, "[doDeployOneWorkload] failed to deploy workload %s, rollback", workload.ID)
			if workload.ID == "" {
				return nil
			}

			if err := c.store.RemoveWorkload(ctx, workload); err != nil {
				logger.Errorf(ctx, "[doDeployOneWorkload] failed to remove workload %s", workload.ID)
			}

			return workload.Remove(ctx, true)
		},
		c.config.GlobalTimeout,
	)
}

func (c *Calcium) doMakeWorkloadOptions(ctx context.Context, no int, msg *types.CreateWorkloadMessage, opts *types.DeployOptions, node *types.Node) *enginetypes.VirtualizationCreateOptions {
	config := &enginetypes.VirtualizationCreateOptions{}
	// general
	config.EngineArgs = msg.EngineArgs
	config.RawArgs = opts.RawArgs
	config.Lambda = opts.Lambda
	config.User = opts.User
	config.DNS = opts.DNS
	config.Image = opts.Image
	config.Stdin = opts.OpenStdin
	config.Hosts = opts.ExtraHosts
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
		config.LogConfig = map[string]string{}
		for k, v := range entry.Log.Config {
			config.LogConfig[k] = v
		}
	}
	// name
	suffix := utils.RandomString(6)
	config.Name = utils.MakeWorkloadName(opts.Name, opts.Entrypoint.Name, suffix)
	msg.WorkloadName = config.Name
	// command and user
	// extra args is dynamically
	config.Cmd = opts.Entrypoint.Commands
	// env
	env := append(opts.Env, fmt.Sprintf("APP_NAME=%s", opts.Name)) // nolint
	env = append(env, fmt.Sprintf("ERU_POD=%s", opts.Podname))
	env = append(env, fmt.Sprintf("ERU_NODE_NAME=%s", node.Name))
	env = append(env, fmt.Sprintf("ERU_WORKLOAD_SEQ=%d", no))
	config.Env = env
	// basic labels, bind to LabelMeta
	config.Labels = map[string]string{
		cluster.ERUMark: "1",
		cluster.LabelMeta: utils.EncodeMetaInLabel(ctx, &types.LabelMeta{
			Publish:     opts.Entrypoint.Publish,
			HealthCheck: entry.HealthCheck,
		}),
		cluster.LabelNodeName: node.Name,
		cluster.LabelCoreID:   c.identifier,
	}
	for key, value := range opts.Labels {
		config.Labels[key] = value
	}

	return config
}
