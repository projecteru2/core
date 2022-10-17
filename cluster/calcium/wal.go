package calcium

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/panjf2000/ants/v2"
	"github.com/pkg/errors"
	"github.com/projecteru2/core/cluster"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/store"
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

func enableWAL(config types.Config, calcium cluster.Cluster, store store.Store) (wal.WAL, error) {
	hydro, err := wal.NewHydro(config.WALFile, config.WALOpenTimeout)
	if err != nil {
		return nil, err
	}

	hydro.Register(newCreateLambdaHandler(config, calcium, store))
	hydro.Register(newCreateWorkloadHandler(config, calcium, store))
	hydro.Register(newWorkloadResourceAllocatedHandler(config, calcium, store))
	hydro.Register(newProcessingCreatedHandler(config, calcium, store))
	return hydro, nil
}

// CreateLambdaHandler indicates event handler for creating lambda.
type CreateLambdaHandler struct {
	typ     string
	config  types.Config
	calcium cluster.Cluster
	store   store.Store
}

func newCreateLambdaHandler(config types.Config, calcium cluster.Cluster, store store.Store) *CreateLambdaHandler {
	return &CreateLambdaHandler{
		typ:     eventCreateLambda,
		config:  config,
		calcium: calcium,
		store:   store,
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
			logger.Errorf(ctx, err, "Get workload failed: %v", err)
			return
		}

		r, err := workload.Engine.VirtualizationWait(ctx, workloadID, "")
		if err != nil {
			logger.Errorf(ctx, err, "Wait failed: %+v", err)
			return
		}
		if r.Code != 0 {
			logger.Errorf(ctx, nil, "Run failed: %s", r.Message)
		}

		if err := h.calcium.RemoveWorkloadSync(ctx, []string{workloadID}); err != nil {
			logger.Errorf(ctx, err, "Remove failed: %+v", err)
		}
		logger.Infof(ctx, "waited and removed")
	}()

	return nil
}

// CreateWorkloadHandler indicates event handler for creating workload.
type CreateWorkloadHandler struct {
	typ     string
	config  types.Config
	calcium cluster.Cluster
	store   store.Store
}

func newCreateWorkloadHandler(config types.Config, calcium cluster.Cluster, store store.Store) *CreateWorkloadHandler {
	return &CreateWorkloadHandler{
		typ:     eventWorkloadCreated,
		config:  config,
		calcium: calcium,
		store:   store,
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
		return h.calcium.RemoveWorkloadSync(ctx, []string{wrk.ID})
	}

	// workload meta doesn't exist
	node, err := h.calcium.GetNode(ctx, wrk.Nodename)
	if err != nil {
		logger.Errorf(ctx, err, "")
		return err
	}
	if err = node.Engine.VirtualizationRemove(ctx, wrk.ID, true, true); err != nil && !errors.Is(err, types.ErrWorkloadNotExists) {
		logger.Errorf(ctx, err, "")
		return err
	}

	logger.Infof(ctx, "workload removed")
	return nil
}

// WorkloadResourceAllocatedHandler .
type WorkloadResourceAllocatedHandler struct {
	typ     string
	config  types.Config
	calcium cluster.Cluster
	store   store.Store
	pool    *ants.PoolWithFunc
}

func newWorkloadResourceAllocatedHandler(config types.Config, calcium cluster.Cluster, store store.Store) *WorkloadResourceAllocatedHandler {
	pool, _ := utils.NewPool(config.MaxConcurrency)
	return &WorkloadResourceAllocatedHandler{
		typ:     eventWorkloadResourceAllocated,
		config:  config,
		calcium: calcium,
		store:   store,
		pool:    pool,
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

	wg := &sync.WaitGroup{}
	wg.Add(len(nodes))
	defer wg.Wait()
	for _, node := range nodes {
		node := node
		_ = h.pool.Invoke(func() {
			defer wg.Done()
			if _, err = h.calcium.NodeResource(ctx, node.Name, true); err != nil {
				logger.Errorf(ctx, err, "failed to fix node resource: %s, %+v", node.Name, err)
				return
			}
			logger.Infof(ctx, "fixed node resource: %s", node.Name)
		})
	}

	return nil
}

// ProcessingCreatedHandler .
type ProcessingCreatedHandler struct {
	typ     string
	config  types.Config
	calcium cluster.Cluster
	store   store.Store
}

func newProcessingCreatedHandler(config types.Config, calcium cluster.Cluster, store store.Store) *ProcessingCreatedHandler {
	return &ProcessingCreatedHandler{
		typ:     eventProcessingCreated,
		config:  config,
		calcium: calcium,
		store:   store,
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

	if err = h.store.DeleteProcessing(ctx, processing); err != nil {
		logger.Errorf(ctx, err, "")
		return err
	}
	logger.Infof(ctx, "obsolete processing deleted")
	return
}

func getReplayContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, time.Second*32) // TODO why 32?
}
