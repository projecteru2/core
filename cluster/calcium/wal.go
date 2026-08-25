package calcium

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/cockroachdb/errors"

	"github.com/projecteru2/core/cluster"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/store"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/wal"
)

const (
	eventCreateLambda              = "create-lambda"
	eventWorkloadCreated           = "create-workload"   // created but yet to start
	eventWorkloadResourceAllocated = "allocate-workload" // resource updated in node meta but yet to create all workloads
	eventProcessingCreated         = "create-processing" // processing created but yet to delete

	replayTimeout = 32 * time.Second
)

// CreateLambdaHandler waits for a replayed lambda workload and removes it.
type CreateLambdaHandler struct {
	calcium cluster.Cluster
}

func newCreateLambdaHandler(calcium cluster.Cluster) *CreateLambdaHandler {
	return &CreateLambdaHandler{calcium: calcium}
}

func (h *CreateLambdaHandler) Typ() string {
	return eventCreateLambda
}

func (h *CreateLambdaHandler) Encode(raw any) ([]byte, error) {
	workloadID, ok := raw.(string)
	if !ok {
		return nil, errors.Wrapf(types.ErrInvalidWALDataType, "%+v", raw)
	}
	return []byte(workloadID), nil
}

func (h *CreateLambdaHandler) Decode(bs []byte) (any, error) {
	return string(bs), nil
}

func (h *CreateLambdaHandler) Handle(ctx context.Context, raw any) error {
	workloadID, ok := raw.(string)
	if !ok {
		return errors.Wrapf(types.ErrInvalidWALDataType, "%+v", raw)
	}

	logger := log.WithFunc("calcium.CreateLambdaHandler.Handle").WithField("ID", workloadID)
	go func() {
		workload, err := h.calcium.GetWorkload(ctx, workloadID)
		if err != nil {
			logger.Error(ctx, err, "get workload failed")
			return
		}

		r, err := workload.Engine.VirtualizationWait(ctx, workloadID, "")
		if err != nil {
			logger.Error(ctx, err, "wait failed")
			return
		}
		if r.Code != 0 {
			logger.Warnf(ctx, "lambda run failed: %s", r.Message)
		}

		if err := h.calcium.RemoveWorkloadSync(ctx, []string{workloadID}); err != nil {
			logger.Error(ctx, err, "remove failed")
		}
		logger.Info(ctx, "waited and removed")
	}()

	return nil
}

// CreateWorkloadHandler removes a workload left behind by an interrupted create.
type CreateWorkloadHandler struct {
	walBase[*types.Workload]

	calcium cluster.Cluster
}

func newCreateWorkloadHandler(calcium cluster.Cluster) *CreateWorkloadHandler {
	return &CreateWorkloadHandler{calcium: calcium}
}

func (h *CreateWorkloadHandler) Typ() string {
	return eventWorkloadCreated
}

func (h *CreateWorkloadHandler) Handle(ctx context.Context, raw any) (err error) {
	wrk, _ := raw.(*types.Workload)
	logger := log.WithFunc("calcium.CreateWorkloadHandler.Handle").WithField("ID", wrk.ID).WithField("node", wrk.Nodename)

	ctx, cancel := getReplayContext(ctx)
	defer cancel()

	if _, err = h.calcium.GetWorkload(ctx, wrk.ID); err == nil {
		return h.calcium.RemoveWorkloadSync(ctx, []string{wrk.ID})
	}

	node, err := h.calcium.GetNode(ctx, wrk.Nodename)
	if err != nil {
		logger.Error(ctx, err)
		return err
	}
	if err = node.Engine.VirtualizationRemove(ctx, wrk.ID, true, true); err != nil && !errors.Is(err, types.ErrWorkloadNotExists) {
		logger.Error(ctx, err)
		return err
	}

	logger.Info(ctx, "workload removed")
	return nil
}

// WorkloadResourceAllocatedHandler replays a dangling resource allocation by refreshing node resources.
type WorkloadResourceAllocatedHandler struct {
	walBase[[]*types.Node]

	calcium cluster.Cluster
}

func newWorkloadResourceAllocatedHandler(calcium cluster.Cluster) *WorkloadResourceAllocatedHandler {
	return &WorkloadResourceAllocatedHandler{calcium: calcium}
}

func (h *WorkloadResourceAllocatedHandler) Typ() string {
	return eventWorkloadResourceAllocated
}

func (h *WorkloadResourceAllocatedHandler) Handle(ctx context.Context, raw any) error {
	nodes, _ := raw.([]*types.Node)
	logger := log.WithFunc("calcium.WorkloadResourceAllocatedHandler.Handle").WithField("event", eventWorkloadResourceAllocated)

	ctx, cancel := getReplayContext(ctx)
	defer cancel()

	wg := &sync.WaitGroup{}
	defer wg.Wait()
	for _, node := range nodes {
		wg.Go(func() {
			if _, e := h.calcium.NodeResource(ctx, node.Name, true); e != nil {
				logger.Errorf(ctx, e, "failed to fix node resource: %s", node.Name)
				return
			}
			logger.Infof(ctx, "fixed node resource: %s", node.Name)
		})
	}

	return nil
}

// ProcessingCreatedHandler deletes processing records left by an interrupted deploy.
type ProcessingCreatedHandler struct {
	walBase[*types.Processing]

	store store.Store
}

func newProcessingCreatedHandler(store store.Store) *ProcessingCreatedHandler {
	return &ProcessingCreatedHandler{store: store}
}

func (h *ProcessingCreatedHandler) Typ() string {
	return eventProcessingCreated
}

func (h *ProcessingCreatedHandler) Handle(ctx context.Context, raw any) (err error) {
	processing, _ := raw.(*types.Processing)
	logger := log.WithFunc("calcium.ProcessingCreatedHandler.Handle").WithField("event", eventProcessingCreated).WithField("ident", processing.Ident)

	ctx, cancel := getReplayContext(ctx)
	defer cancel()

	if err = h.store.DeleteProcessing(ctx, processing); err != nil {
		logger.Error(ctx, err)
		return err
	}
	logger.Info(ctx, "obsolete processing deleted")
	return err
}

type walBase[T any] struct{}

func (walBase[T]) Encode(raw any) ([]byte, error) {
	v, ok := raw.(T)
	if !ok {
		return nil, errors.Wrapf(types.ErrInvalidWALDataType, "%+v", raw)
	}
	return json.Marshal(v)
}

func (walBase[T]) Decode(bs []byte) (any, error) {
	var v T
	err := json.Unmarshal(bs, &v)
	return v, err
}

func enableWAL(config types.Config, calcium cluster.Cluster, store store.Store) (wal.WAL, error) {
	hydro, err := wal.NewHydro(config.WALFile, config.WALOpenTimeout)
	if err != nil {
		return nil, err
	}

	hydro.Register(newCreateLambdaHandler(calcium))
	hydro.Register(newCreateWorkloadHandler(calcium))
	hydro.Register(newWorkloadResourceAllocatedHandler(calcium))
	hydro.Register(newProcessingCreatedHandler(store))
	return hydro, nil
}

func getReplayContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, replayTimeout)
}
