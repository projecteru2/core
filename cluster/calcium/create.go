package calcium

import (
	"cmp"
	"context"
	"fmt"
	"maps"
	"runtime"
	"slices"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/sanity-io/litter"
	"golang.org/x/sync/errgroup"

	"github.com/projecteru2/core/cluster"
	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/metrics"
	resourcetypes "github.com/projecteru2/core/resource/types"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

type createEmit func(*types.CreateWorkloadMessage)

type deployPlan struct {
	deploy            map[string]int
	engineParams      map[string][]resourcetypes.Resources
	workloadResources map[string][]resourcetypes.Resources
}

type nodeDeploy struct {
	nodename          string
	deploy            int
	seq               int
	engineParams      []resourcetypes.Resources
	workloadResources []resourcetypes.Resources
}

func (c *Calcium) CreateWorkload(ctx context.Context, opts *types.DeployOptions) (chan *types.CreateWorkloadMessage, error) {
	logger := log.WithFunc("calcium.CreateWorkload").WithField("opts", opts)
	if err := opts.Validate(); err != nil {
		logger.Error(ctx, err)
		return nil, err
	}
	opts.ProcessIdent = utils.RandomString(16)
	logger = logger.WithField("ident", opts.ProcessIdent)
	logger.Infof(ctx, "creating workload ident %s with options:\n%s", opts.ProcessIdent, litter.Options{Compact: true}.Sdump(opts))
	if opts.Count <= 0 {
		err := errors.Wrapf(types.ErrInvaildDeployCount, "count: %d", opts.Count)
		logger.Error(ctx, err)
		return nil, err
	}

	return c.doCreateWorkloads(ctx, opts), nil
}

func (c *Calcium) doCreateWorkloads(ctx context.Context, opts *types.DeployOptions) chan *types.CreateWorkloadMessage {
	logger := log.WithFunc("calcium.doCreateWorkloads").WithField("ident", opts.ProcessIdent)
	ch := make(chan *types.CreateWorkloadMessage)

	var (
		rollbackMap map[string][]int
		plan        = deployPlan{
			engineParams:      map[string][]resourcetypes.Resources{},
			workloadResources: map[string][]resourcetypes.Resources{},
		}
	)

	caller := ctx
	utils.SentryGo(func() {
		var resourceCommit func()
		var processingCommits map[string]func()
		defer func() {
			cctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), c.config.GlobalTimeout)
			defer cancel()
			for nodename := range plan.deploy {
				processing := opts.GetProcessing(nodename)
				if err := c.store.DeleteProcessing(cctx, processing); err != nil {
					logger.Errorf(ctx, err, "delete processing failed for %s", nodename)
					continue
				}
				if commit := processingCommits[nodename]; commit != nil {
					commit()
				}
			}
			close(ch)
		}()

		settled, _ := utils.Txn(
			ctx,

			func(ctx context.Context) (err error) {
				defer func() {
					if err != nil {
						logger.Error(ctx, err)
						_ = send(caller, ch, &types.CreateWorkloadMessage{Error: err})
					}
				}()
				return c.withNodesPlanLocked(ctx, opts.NodeFilter, func(ctx context.Context, nodeMap map[string]*types.Node) (err error) {
					if len(nodeMap) == 0 {
						return types.ErrEmptyNodeMap
					}
					nodenames := slices.Collect(maps.Keys(nodeMap))
					nodes := slices.Collect(maps.Values(nodeMap))

					if resourceCommit, err = c.journal(ctx, logger, eventWorkloadResourceAllocated, nodes); err != nil {
						return err
					}

					plan.deploy, err = c.doGetDeployStrategy(ctx, nodenames, opts)
					if err != nil {
						return err
					}

					processingCommits = make(map[string]func())
					mu := sync.Mutex{}
					allocs := errgroup.Group{}
					allocs.SetLimit(runtime.GOMAXPROCS(0))
					for nodename, deploy := range plan.deploy {
						allocs.Go(func() error {
							workloadResources, engineParams, allocErr := c.rmgr.Alloc(ctx, nodename, deploy, opts.Resources)
							if allocErr != nil {
								return allocErr
							}
							processing := opts.GetProcessing(nodename)
							commit, journalErr := c.journal(ctx, logger, eventProcessingCreated, processing)
							mu.Lock()
							plan.workloadResources[nodename] = workloadResources
							plan.engineParams[nodename] = engineParams
							processingCommits[nodename] = commit
							mu.Unlock()
							if journalErr != nil {
								return journalErr
							}
							return c.store.CreateProcessing(ctx, processing, deploy)
						})
					}
					return allocs.Wait()
				})
			},

			func(ctx context.Context) (err error) {
				rollbackMap, err = c.doDeployWorkloads(ctx, func(msg *types.CreateWorkloadMessage) { _ = send(caller, ch, msg) }, opts, plan)
				return err
			},

			func(ctx context.Context, failedOnCond bool) (err error) {
				resourcesToRollback := map[string][]resourcetypes.Resources{}
				if failedOnCond {
					resourcesToRollback = plan.workloadResources
				} else {
					for nodename, rollbackIndices := range rollbackMap {
						resourcesToRollback[nodename] = utils.Map(rollbackIndices, func(idx int) resourcetypes.Resources {
							return plan.workloadResources[nodename][idx]
						})
					}
				}
				for nodename, rollbackResources := range resourcesToRollback {
					if e := c.withNodeOperationLocked(ctx, nodename, func(ctx context.Context, _ *types.Node) error {
						return c.rmgr.RollbackAlloc(ctx, nodename, rollbackResources)
					}); e != nil {
						logger.Error(ctx, e)
						err = errors.Join(err, e)
					}
				}
				return err
			},

			c.config.GlobalTimeout,
		)
		if resourceCommit != nil && settled {
			resourceCommit()
		}
	})

	return ch
}

func (c *Calcium) doDeployWorkloads(ctx context.Context, emit createEmit, opts *types.DeployOptions, plan deployPlan) (_ map[string][]int, err error) {
	wg := sync.WaitGroup{}
	logger := log.WithFunc("calcium.doDeployWorkloads").WithField("ident", opts.ProcessIdent)

	total := 0
	for _, deploy := range plan.deploy {
		total += deploy
	}
	c.invokePoolAsync(func() { metrics.Client.SendDeployCount(ctx, total) })

	seq := 0
	rollbackLock := sync.Mutex{}
	rollbackMap := make(map[string][]int)
	for nodename, deploy := range plan.deploy {
		nd := nodeDeploy{
			nodename:          nodename,
			deploy:            deploy,
			seq:               seq,
			engineParams:      plan.engineParams[nodename],
			workloadResources: plan.workloadResources[nodename],
		}
		seq += deploy
		wg.Go(func() {
			defer log.SentryDefer()
			if indices, deployErr := c.doDeployWorkloadsOnNode(ctx, emit, opts, nd); deployErr != nil {
				rollbackLock.Lock()
				rollbackMap[nodename] = indices
				rollbackLock.Unlock()
			}
		})
	}

	wg.Wait()
	logger.Debugf(ctx, "rollbackMap: %+v", rollbackMap)
	if len(rollbackMap) != 0 {
		err = types.ErrRollbackMapIsNotEmpty
	}
	return rollbackMap, err
}

func (c *Calcium) doDeployWorkloadsOnNode(ctx context.Context, emit createEmit, opts *types.DeployOptions, nd nodeDeploy) (indices []int, err error) {
	logger := log.WithFunc("calcium.doDeployWorkloadsOnNode").WithField("node", nd.nodename).WithField("ident", opts.ProcessIdent).WithField("deploy", nd.deploy).WithField("seq", nd.seq)
	node, err := c.doGetAndPrepareNode(ctx, nd.nodename, opts.Image, opts.IgnorePull)
	if err != nil {
		logger.Error(ctx, err)
		for range nd.deploy {
			emit(&types.CreateWorkloadMessage{Error: err})
		}
		return utils.Range(nd.deploy), err
	}

	appendLock := sync.Mutex{}
	wg := &sync.WaitGroup{}
	wg.Add(nd.deploy)
	for idx := range nd.deploy {
		createMsg := &types.CreateWorkloadMessage{
			Podname:  opts.Podname,
			Nodename: nd.nodename,
			Publish:  map[string][]string{},
		}

		_ = c.pool.Invoke(func() {
			defer wg.Done()
			var e error
			defer func() {
				if e != nil {
					logger.Error(ctx, e)
					createMsg.Error = e
					appendLock.Lock()
					err = e
					indices = append(indices, idx)
					appendLock.Unlock()
				}
				emit(createMsg)
			}()

			createMsg.EngineParams = nd.engineParams[idx]
			createMsg.Resources = nd.workloadResources[idx]

			createOpts := c.doMakeWorkloadOptions(ctx, nd.seq+idx, createMsg, opts, node)
			e = c.doDeployOneWorkload(ctx, node, opts, createMsg, createOpts, true)
		})
	}
	wg.Wait()

	c.invokePoolAsync(func() { c.RemapResourceAndLog(ctx, logger, node.Name) })

	return indices, err
}

func (c *Calcium) doGetAndPrepareNode(ctx context.Context, nodename, image string, ignorePull bool) (*types.Node, error) {
	node, err := c.store.GetNode(ctx, nodename)
	if err != nil {
		return nil, err
	}
	if !ignorePull {
		err = pullImage(ctx, node, image)
	}

	return node, err
}

func (c *Calcium) doDeployOneWorkload(ctx context.Context, node *types.Node, opts *types.DeployOptions, msg *types.CreateWorkloadMessage, createOpts *enginetypes.VirtualizationCreateOptions, decrProcessing bool) (err error) {
	logger := log.WithFunc("calcium.doDeployOneWorkload").WithField("node", node.Name).WithField("ident", opts.ProcessIdent).WithField("msg", msg)
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

	commit, err := c.journal(ctx, logger, eventWorkloadCreated, &types.Workload{
		Name:     workload.Name,
		Nodename: workload.Nodename,
	})
	if err != nil {
		return err
	}

	settled, txnErr := utils.Txn(
		ctx,
		func(ctx context.Context) error {
			created, err := node.Engine.VirtualizationCreate(ctx, createOpts)
			if err != nil {
				return err
			}
			workload.ID = created.ID

			maps.Copy(workload.Labels, created.Labels)
			return nil
		},

		func(ctx context.Context) (err error) {
			processing := opts.GetProcessing(node.Name)
			if !decrProcessing {
				processing = nil
			}
			if err = c.store.AddWorkload(ctx, workload, processing); err != nil {
				return err
			}
			logger.Infof(ctx, "workload %s metadata created", workload.ID)

			if len(opts.Files) > 0 {
				for _, file := range opts.Files {
					if err = c.doSendFileToWorkload(ctx, node.Engine, workload.ID, file); err != nil {
						return err
					}
				}
			}

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

			msg.Hook, err = c.doStartWorkload(ctx, workload, opts.IgnoreHook)
			if err != nil {
				return err
			}

			workload.Hook = opts.Entrypoint.Hook

			var workloadInfo *enginetypes.VirtualizationInfo
			workloadInfo, err = workload.Inspect(ctx)
			if err != nil {
				return err
			}

			if workloadInfo.Networks != nil {
				msg.Publish = utils.MakePublishInfo(workloadInfo.Networks, opts.Entrypoint.Publish)
			}

			if workloadInfo.User != workload.User {
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

		func(ctx context.Context, _ bool) (rollbackErr error) {
			logger.Warnf(ctx, "failed to deploy workload %s, rollback", cmp.Or(workload.ID, workload.Name))
			if workload.ID == "" {
				return removeWorkloadByName(ctx, node, workload.Name)
			}

			if removeErr := c.store.RemoveWorkload(ctx, workload); removeErr != nil {
				logger.Errorf(ctx, removeErr, "failed to remove workload %s", workload.ID)
				rollbackErr = errors.Join(rollbackErr, removeErr)
			}

			return errors.Join(rollbackErr, workload.Remove(ctx, true))
		},
		c.config.GlobalTimeout,
	)
	if settled {
		commit()
	}
	return txnErr
}

func (c *Calcium) doMakeWorkloadOptions(ctx context.Context, no int, msg *types.CreateWorkloadMessage, opts *types.DeployOptions, node *types.Node) *enginetypes.VirtualizationCreateOptions {
	createOpts := &enginetypes.VirtualizationCreateOptions{}
	createOpts.EngineParams = msg.EngineParams
	createOpts.RawArgs = opts.RawArgs
	createOpts.Lambda = opts.Lambda
	createOpts.User = opts.User
	createOpts.DNS = opts.DNS
	createOpts.Image = opts.Image
	createOpts.Stdin = opts.OpenStdin
	createOpts.Hosts = opts.ExtraHosts
	createOpts.Networks = opts.Networks

	entry := opts.Entrypoint
	createOpts.WorkingDir = entry.Dir
	createOpts.Privileged = entry.Privileged
	createOpts.Sysctl = entry.Sysctls
	createOpts.Restart = entry.Restart
	suffix := utils.RandomString(6)
	createOpts.Name = utils.MakeWorkloadName(opts.Name, opts.Entrypoint.Name, suffix)
	msg.WorkloadName = createOpts.Name
	createOpts.Cmd = opts.Entrypoint.Commands
	createOpts.Env = slices.Concat(opts.Env, []string{
		fmt.Sprintf("APP_NAME=%s", opts.Name),
		fmt.Sprintf("ERU_POD=%s", opts.Podname),
		fmt.Sprintf("ERU_NODE_NAME=%s", node.Name),
		fmt.Sprintf("ERU_WORKLOAD_SEQ=%d", no),
	})
	createOpts.Labels = map[string]string{
		cluster.ERUMark: "1",
		cluster.LabelMeta: utils.EncodeMetaInLabel(ctx, &types.LabelMeta{
			Publish:     opts.Entrypoint.Publish,
			HealthCheck: entry.HealthCheck,
		}),
		cluster.LabelNodeName: node.Name,
		cluster.LabelCoreID:   c.identifier,
	}
	maps.Copy(createOpts.Labels, opts.Labels)

	return createOpts
}
