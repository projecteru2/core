package calcium

import (
	"context"
	"encoding/json"
	"slices"
	"sync"
	"time"

	"github.com/cockroachdb/errors"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/store"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/wal"
)

const (
	eventCreateLambda              = "create-lambda"
	eventWorkloadCreated           = "create-workload" // created but yet to start
	eventWorkloadReplaced          = "replace-workload"
	eventWorkloadReallocated       = "realloc-workload"
	eventWorkloadResourceAllocated = "allocate-workload" // resource updated in node meta but yet to create all workloads
	eventProcessingCreated         = "create-processing" // processing created but yet to delete

	replayTimeout = 32 * time.Second
)

// CreateLambdaHandler waits for a replayed lambda workload and removes it.
type CreateLambdaHandler struct {
	calcium *Calcium
	wal     wal.WAL
}

func newCreateLambdaHandler(calcium *Calcium, wal wal.WAL) *CreateLambdaHandler {
	return &CreateLambdaHandler{calcium: calcium, wal: wal}
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

	commit, err := h.wal.Log(eventCreateLambda, workloadID)
	if err != nil {
		return err
	}

	logger := log.WithFunc("calcium.CreateLambdaHandler.Handle").WithField("ID", workloadID)
	go func() {
		if err := h.waitAndRemove(ctx, logger, workloadID); err != nil {
			logger.Error(ctx, err, "wait and remove failed")
			return
		}
		if err := commit(); err != nil {
			logger.Errorf(ctx, err, "commit wal %s failed", eventCreateLambda)
		}
		logger.Info(ctx, "waited and removed")
	}()

	return nil
}

func (h *CreateLambdaHandler) waitAndRemove(ctx context.Context, logger *log.Fields, workloadID string) error {
	workload, err := getWorkloadIfExists(ctx, h.calcium, workloadID)
	if err != nil || workload == nil {
		return err
	}

	r, err := workload.Engine.VirtualizationWait(ctx, workloadID, "")
	if err != nil {
		return err
	}
	if r.Code != 0 {
		logger.Warnf(ctx, "lambda run failed: %s", r.Message)
	}
	return h.calcium.RemoveWorkloadSync(ctx, []string{workloadID})
}

// CreateWorkloadHandler removes a workload left behind by an interrupted create.
type CreateWorkloadHandler struct {
	walBase[*types.Workload]

	calcium *Calcium
}

func newCreateWorkloadHandler(calcium *Calcium) *CreateWorkloadHandler {
	return &CreateWorkloadHandler{calcium: calcium}
}

func (h *CreateWorkloadHandler) Typ() string {
	return eventWorkloadCreated
}

func (h *CreateWorkloadHandler) Handle(ctx context.Context, raw any) error {
	wrk, _ := raw.(*types.Workload)
	logger := log.WithFunc("calcium.CreateWorkloadHandler.Handle").WithField("ID", wrk.ID).WithField("name", wrk.Name).WithField("node", wrk.Nodename)

	ctx, cancel := getReplayContext(ctx)
	defer cancel()

	storedID, err := h.storedWorkloadID(ctx, wrk)
	if err != nil {
		logger.Error(ctx, err)
		return err
	}
	if storedID != "" {
		return h.calcium.RemoveWorkloadSync(ctx, []string{storedID})
	}

	node, err := h.calcium.GetNode(ctx, wrk.Nodename)
	if err != nil {
		if h.calcium.store.NotFound(err) {
			logger.Info(ctx, "node is gone, nothing to remove")
			return nil
		}
		logger.Error(ctx, err)
		return err
	}

	if err = h.removeFromEngine(ctx, node, wrk); err != nil {
		logger.Error(ctx, err)
		return err
	}

	logger.Info(ctx, "workload removed")
	return nil
}

func (h *CreateWorkloadHandler) storedWorkloadID(ctx context.Context, wrk *types.Workload) (string, error) {
	if wrk.ID != "" {
		if _, err := h.calcium.GetWorkload(ctx, wrk.ID); err != nil {
			return "", nil
		}
		return wrk.ID, nil
	}

	workloads, err := h.calcium.store.ListNodeWorkloads(ctx, wrk.Nodename, nil)
	if err != nil {
		return "", err
	}
	index := slices.IndexFunc(workloads, func(workload *types.Workload) bool { return workload.Name == wrk.Name })
	if index < 0 {
		return "", nil
	}
	return workloads[index].ID, nil
}

func (h *CreateWorkloadHandler) removeFromEngine(ctx context.Context, node *types.Node, wrk *types.Workload) error {
	if wrk.ID == "" {
		return removeWorkloadByName(ctx, node, wrk.Name)
	}
	if err := node.Engine.VirtualizationRemove(ctx, wrk.ID, true, true); err != nil && !errors.Is(err, types.ErrWorkloadNotExists) {
		return err
	}
	return nil
}

type workloadReplacement struct {
	OldID string `json:"old_id"`
	NewID string `json:"new_id"`
}

// ReplaceWorkloadHandler removes the workload an interrupted replace left behind.
type ReplaceWorkloadHandler struct {
	walBase[*workloadReplacement]

	calcium *Calcium
}

func newReplaceWorkloadHandler(calcium *Calcium) *ReplaceWorkloadHandler {
	return &ReplaceWorkloadHandler{calcium: calcium}
}

func (h *ReplaceWorkloadHandler) Typ() string {
	return eventWorkloadReplaced
}

func (h *ReplaceWorkloadHandler) Handle(ctx context.Context, raw any) error {
	replacement, _ := raw.(*workloadReplacement)
	logger := log.WithFunc("calcium.ReplaceWorkloadHandler.Handle").WithField("ID", replacement.OldID)

	ctx, cancel := getReplayContext(ctx)
	defer cancel()

	newWorkload, err := getWorkloadIfExists(ctx, h.calcium, replacement.NewID)
	if err != nil || newWorkload == nil {
		return err
	}

	oldWorkload, err := getWorkloadIfExists(ctx, h.calcium, replacement.OldID)
	if err != nil || oldWorkload == nil {
		return err
	}

	if err = h.calcium.doRemoveWorkload(ctx, oldWorkload, true); err != nil {
		logger.Error(ctx, err)
		return err
	}
	logger.Info(ctx, "replaced workload removed")
	return nil
}

// ReallocWorkloadHandler re-applies the stored engine params of an interrupted realloc.
type ReallocWorkloadHandler struct {
	walBase[string]

	calcium *Calcium
}

func newReallocWorkloadHandler(calcium *Calcium) *ReallocWorkloadHandler {
	return &ReallocWorkloadHandler{calcium: calcium}
}

func (h *ReallocWorkloadHandler) Typ() string {
	return eventWorkloadReallocated
}

func (h *ReallocWorkloadHandler) Handle(ctx context.Context, raw any) error {
	workloadID, _ := raw.(string)
	logger := log.WithFunc("calcium.ReallocWorkloadHandler.Handle").WithField("ID", workloadID)

	ctx, cancel := getReplayContext(ctx)
	defer cancel()

	workload, err := getWorkloadIfExists(ctx, h.calcium, workloadID)
	if err != nil || workload == nil {
		return err
	}

	if err = workload.Engine.VirtualizationUpdateResource(ctx, workloadID, workload.EngineParams); err != nil {
		logger.Error(ctx, err)
		return err
	}
	logger.Info(ctx, "engine params reapplied")
	return nil
}

// WorkloadResourceAllocatedHandler replays a dangling resource allocation by refreshing node resources.
type WorkloadResourceAllocatedHandler struct {
	walBase[[]*types.Node]

	calcium *Calcium
}

func newWorkloadResourceAllocatedHandler(calcium *Calcium) *WorkloadResourceAllocatedHandler {
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

func enableWAL(config types.Config, calcium *Calcium, store store.Store) (wal.WAL, error) {
	hydro, err := wal.NewHydro(config.WALFile, config.WALOpenTimeout)
	if err != nil {
		return nil, err
	}

	hydro.Register(newCreateLambdaHandler(calcium, hydro))
	hydro.Register(newCreateWorkloadHandler(calcium))
	hydro.Register(newReplaceWorkloadHandler(calcium))
	hydro.Register(newReallocWorkloadHandler(calcium))
	hydro.Register(newWorkloadResourceAllocatedHandler(calcium))
	hydro.Register(newProcessingCreatedHandler(store))
	return hydro, nil
}

func getReplayContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, replayTimeout)
}

func getWorkloadIfExists(ctx context.Context, calcium *Calcium, ID string) (*types.Workload, error) {
	workload, err := calcium.GetWorkload(ctx, ID)
	if err != nil && calcium.store.NotFound(err) {
		return nil, nil
	}
	return workload, err
}
