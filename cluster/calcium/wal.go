package calcium

import (
	"context"
	"encoding/json"
	"time"

	"github.com/pkg/errors"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
	"github.com/projecteru2/core/wal"
)

const (
	eventCreateLambda              = "create-lambda"
	eventWorkloadCreated           = "create-workload"   // created but yet to start
	eventWorkloadResourceAllocated = "allocate-workload" // resource updated in node meta but yet to create all workloads
	eventProcessingCreated         = "create-processing" // processing created but yet to delete
)

// WAL for calcium.
type WAL struct {
	wal.WAL
	calcium *Calcium
}

func newWAL(config types.Config, calcium *Calcium) (*WAL, error) {
	hydro, err := wal.NewHydro(config.WALFile, config.WALOpenTimeout)
	if err != nil {
		return nil, err
	}

	w := &WAL{
		WAL:     hydro,
		calcium: calcium,
	}

	w.registerHandlers()
	return w, nil
}

func (w *WAL) registerHandlers() {
	w.Register(newCreateLambdaHandler(w.calcium))
	w.Register(newCreateWorkloadHandler(w.calcium))
	w.Register(newWorkloadResourceAllocatedHandler(w.calcium))
	w.Register(newProcessingCreatedHandler(w.calcium))
}

// CreateLambdaHandler indicates event handler for creating lambda.
type CreateLambdaHandler struct {
	typ     string
	calcium *Calcium
}

func newCreateLambdaHandler(calcium *Calcium) *CreateLambdaHandler {
	return &CreateLambdaHandler{
		typ:     eventCreateLambda,
		calcium: calcium,
	}
}

// Event .
func (h *CreateLambdaHandler) Typ() string {
	return h.typ
}

// Check .
func (h *CreateLambdaHandler) Check(context.Context, interface{}) (bool, error) {
	return true, nil
}

// Encode .
func (h *CreateLambdaHandler) Encode(raw interface{}) ([]byte, error) {
	workloadID, ok := raw.(string)
	if !ok {
		return nil, types.NewDetailedErr(types.ErrInvalidType, raw)
	}
	return []byte(workloadID), nil
}

// Decode .
func (h *CreateLambdaHandler) Decode(bs []byte) (interface{}, error) {
	return string(bs), nil
}

// Handle .
func (h *CreateLambdaHandler) Handle(ctx context.Context, raw interface{}) error {
	workloadID, ok := raw.(string)
	if !ok {
		return types.NewDetailedErr(types.ErrInvalidType, raw)
	}

	logger := log.WithField("WAL.Handle", "RunAndWait").WithField("ID", workloadID)
	go func() {
		workload, err := h.calcium.GetWorkload(ctx, workloadID)
		if err != nil {
			logger.Errorf(ctx, "Get workload failed: %v", err)
			return
		}

		r, err := workload.Engine.VirtualizationWait(ctx, workloadID, "")
		if err != nil {
			logger.Errorf(ctx, "Wait failed: %+v", err)
			return
		}
		if r.Code != 0 {
			logger.Errorf(ctx, "Run failed: %s", r.Message)
		}

		if err := h.calcium.doRemoveWorkloadSync(ctx, []string{workloadID}); err != nil {
			logger.Errorf(ctx, "Remove failed: %+v", err)
		}
		logger.Infof(ctx, "waited and removed")
	}()

	return nil
}

// CreateWorkloadHandler indicates event handler for creating workload.
type CreateWorkloadHandler struct {
	typ     string
	calcium *Calcium
}

func newCreateWorkloadHandler(calcium *Calcium) *CreateWorkloadHandler {
	return &CreateWorkloadHandler{
		typ:     eventWorkloadCreated,
		calcium: calcium,
	}
}

// Event .
func (h *CreateWorkloadHandler) Typ() string {
	return h.typ
}

// Check .
func (h *CreateWorkloadHandler) Check(ctx context.Context, raw interface{}) (handle bool, err error) {
	_, ok := raw.(*types.Workload)
	if !ok {
		return false, types.NewDetailedErr(types.ErrInvalidType, raw)
	}
	return true, nil
}

// Encode .
func (h *CreateWorkloadHandler) Encode(raw interface{}) ([]byte, error) {
	wrk, ok := raw.(*types.Workload)
	if !ok {
		return nil, types.NewDetailedErr(types.ErrInvalidType, raw)
	}
	return json.Marshal(wrk)
}

// Decode .
func (h *CreateWorkloadHandler) Decode(bs []byte) (interface{}, error) {
	wrk := &types.Workload{}
	err := json.Unmarshal(bs, wrk)
	return wrk, err
}

// Handle will remove instance, remove meta, restore resource
func (h *CreateWorkloadHandler) Handle(ctx context.Context, raw interface{}) (err error) {
	wrk, _ := raw.(*types.Workload)
	logger := log.WithField("WAL.Handle", "CreateWorkload").WithField("ID", wrk.ID).WithField("nodename", wrk.Nodename)

	ctx, cancel := getReplayContext(ctx)
	defer cancel()

	if _, err = h.calcium.GetWorkload(ctx, wrk.ID); err == nil {
		return h.calcium.doRemoveWorkloadSync(ctx, []string{wrk.ID})
	}

	// workload meta doesn't exist
	node, err := h.calcium.GetNode(ctx, wrk.Nodename)
	if err != nil {
		return logger.Err(ctx, err)
	}
	if err = node.Engine.VirtualizationRemove(ctx, wrk.ID, true, true); err != nil && !errors.Is(err, types.ErrWorkloadNotExists) {
		return logger.Err(ctx, err)
	}

	logger.Infof(ctx, "workload removed")
	return nil
}

// WorkloadResourceAllocatedHandler .
type WorkloadResourceAllocatedHandler struct {
	typ     string
	calcium *Calcium
}

func newWorkloadResourceAllocatedHandler(calcium *Calcium) *WorkloadResourceAllocatedHandler {
	return &WorkloadResourceAllocatedHandler{
		typ:     eventWorkloadResourceAllocated,
		calcium: calcium,
	}
}

// Event .
func (h *WorkloadResourceAllocatedHandler) Typ() string {
	return h.typ
}

// Check .
func (h *WorkloadResourceAllocatedHandler) Check(ctx context.Context, raw interface{}) (bool, error) {
	if _, ok := raw.([]*types.Node); !ok {
		return false, types.NewDetailedErr(types.ErrInvalidType, raw)
	}
	return true, nil
}

// Encode .
func (h *WorkloadResourceAllocatedHandler) Encode(raw interface{}) ([]byte, error) {
	nodes, ok := raw.([]*types.Node)
	if !ok {
		return nil, types.NewDetailedErr(types.ErrInvalidType, raw)
	}
	return json.Marshal(nodes)
}

// Decode .
func (h *WorkloadResourceAllocatedHandler) Decode(bytes []byte) (interface{}, error) {
	nodes := []*types.Node{}
	return nodes, json.Unmarshal(bytes, &nodes)
}

// Handle .
func (h *WorkloadResourceAllocatedHandler) Handle(ctx context.Context, raw interface{}) (err error) {
	nodes, _ := raw.([]*types.Node)
	logger := log.WithField("WAL", "Handle").WithField("event", eventWorkloadResourceAllocated)

	ctx, cancel := getReplayContext(ctx)
	defer cancel()

	pool := utils.NewGoroutinePool(20)
	for _, node := range nodes {
		pool.Go(ctx, func(nodename string) func() {
			return func() {
				{
					if _, err = h.calcium.NodeResource(ctx, nodename, true); err != nil {
						logger.Errorf(ctx, "failed to fix node resource: %s, %+v", node.Name, err)
						return
					}
					logger.Infof(ctx, "fixed node resource: %s", node.Name)
				}
			}
		}(node.Name))
	}
	pool.Wait(ctx)

	return nil
}

// ProcessingCreatedHandler .
type ProcessingCreatedHandler struct {
	typ     string
	calcium *Calcium
}

func newProcessingCreatedHandler(calcium *Calcium) *ProcessingCreatedHandler {
	return &ProcessingCreatedHandler{
		typ:     eventProcessingCreated,
		calcium: calcium,
	}
}

// Event .
func (h *ProcessingCreatedHandler) Typ() string {
	return h.typ
}

// Check .
func (h ProcessingCreatedHandler) Check(ctx context.Context, raw interface{}) (bool, error) {
	if _, ok := raw.(*types.Processing); !ok {
		return false, types.NewDetailedErr(types.ErrInvalidType, raw)
	}
	return true, nil
}

// Encode .
func (h *ProcessingCreatedHandler) Encode(raw interface{}) ([]byte, error) {
	processing, ok := raw.(*types.Processing)
	if !ok {
		return nil, types.NewDetailedErr(types.ErrInvalidType, raw)
	}
	return json.Marshal(processing)
}

// Decode .
func (h *ProcessingCreatedHandler) Decode(bs []byte) (interface{}, error) {
	processing := &types.Processing{}
	return processing, json.Unmarshal(bs, processing)
}

// Handle .
func (h *ProcessingCreatedHandler) Handle(ctx context.Context, raw interface{}) (err error) {
	processing, _ := raw.(*types.Processing)
	logger := log.WithField("WAL", "Handle").WithField("event", eventProcessingCreated).WithField("ident", processing.Ident)

	ctx, cancel := getReplayContext(ctx)
	defer cancel()

	if err = h.calcium.store.DeleteProcessing(ctx, processing); err != nil {
		return logger.Err(ctx, err)
	}
	logger.Infof(ctx, "obsolete processing deleted")
	return
}

func getReplayContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, time.Second*32) // TODO why 32?
}
