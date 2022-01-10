package calcium

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/wal"
)

const (
	eventCreateLambda   = "create-lambda"
	eventCreateWorkload = "create-workload" // created but yet to start
)

// WAL for calcium.
type WAL struct {
	wal.WAL
	config  types.Config
	calcium *Calcium
}

func newCalciumWAL(cal *Calcium) (*WAL, error) {
	w := &WAL{
		WAL:     wal.NewHydro(),
		config:  cal.config,
		calcium: cal,
	}

	if err := w.WAL.Open(w.config.WALFile, w.config.WALOpenTimeout); err != nil {
		return nil, err
	}

	w.registerHandlers()

	return w, nil
}

func (w *WAL) registerHandlers() {
	w.Register(newCreateLambdaHandler(w.calcium))
	w.Register(newCreateWorkloadHandler(w.calcium))
}

func (w *WAL) logCreateWorkload(workloadID, nodename string) (wal.Commit, error) {
	return w.Log(eventCreateWorkload, &types.Workload{
		ID:       workloadID,
		Nodename: nodename,
	})
}

func (w *WAL) logCreateLambda(opts *types.CreateWorkloadMessage) (wal.Commit, error) {
	return w.Log(eventCreateLambda, opts.WorkloadID)
}

// CreateWorkloadHandler indicates event handler for creating workload.
type CreateWorkloadHandler struct {
	event   string
	calcium *Calcium
}

func newCreateWorkloadHandler(cal *Calcium) *CreateWorkloadHandler {
	return &CreateWorkloadHandler{
		event:   eventCreateWorkload,
		calcium: cal,
	}
}

// Event .
func (h *CreateWorkloadHandler) Event() string {
	return h.event
}

// Check .
func (h *CreateWorkloadHandler) Check(ctx context.Context, raw interface{}) (bool, error) {
	wrk, ok := raw.(*types.Workload)
	if !ok {
		return false, types.NewDetailedErr(types.ErrInvalidType, raw)
	}
	logger := log.WithField("WAL.Check", "CreateWorkload").WithField("ID", wrk.ID)

	ctx, cancel := getReplayContext(ctx)
	defer cancel()

	_, err := h.calcium.GetWorkload(ctx, wrk.ID)
	switch {
	// there has been an exact workload metadata.
	case err == nil:
		return false, nil

	case strings.HasPrefix(err.Error(), types.ErrBadCount.Error()):
		logger.Errorf(ctx, "No such workload")
		return true, nil

	default:
		logger.Errorf(ctx, "Unexpected error: %v", err)
		return false, err
	}
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

// Handle .
func (h *CreateWorkloadHandler) Handle(ctx context.Context, raw interface{}) (err error) {
	wrk, ok := raw.(*types.Workload)
	if !ok {
		return types.NewDetailedErr(types.ErrInvalidType, raw)
	}
	logger := log.WithField("WAL.Handle", "CreateWorkload").WithField("ID", wrk.ID).WithField("nodename", wrk.Nodename)

	ctx, cancel := getReplayContext(ctx)
	defer cancel()

	ch, err := h.calcium.RemoveWorkload(ctx, []string{wrk.ID}, true, 0)
	if err != nil {
		logger.Errorf(ctx, "failed to remove workload")
		return
	}
	for msg := range ch {
		if !msg.Success {
			logger.Errorf(ctx, "failed to remove workload")
			return nil
		}
	}

	logger.Infof(ctx, "workload removed")

	return nil
}

// CreateLambdaHandler indicates event handler for creating lambda.
type CreateLambdaHandler struct {
	event   string
	calcium *Calcium
}

func newCreateLambdaHandler(cal *Calcium) *CreateLambdaHandler {
	return &CreateLambdaHandler{
		event:   eventCreateLambda,
		calcium: cal,
	}
}

// Event .
func (h *CreateLambdaHandler) Event() string {
	return h.event
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
		logger.Infof(ctx, "recovery start")
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

func getReplayContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, time.Second*32)
}
