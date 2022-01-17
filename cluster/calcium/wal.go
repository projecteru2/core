package calcium

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/wal"
)

const (
	eventCreateLambda   = "create-lambda"
	eventCreateWorkload = "create-workload"
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

	ctx, cancel := getReplayContext(ctx)
	defer cancel()

	_, err := h.calcium.GetWorkload(ctx, wrk.ID)
	switch {
	// there has been an exact workload metadata.
	case err == nil:
		log.Infof(ctx, "[CreateWorkloadHandler.Check] Workload %s is availalbe", wrk.ID)
		return false, nil

	case strings.HasPrefix(err.Error(), types.ErrBadCount.Error()):
		log.Errorf(ctx, "[CreateWorkloadHandler.Check] No such workload: %v", wrk.ID)
		return true, nil

	default:
		log.Errorf(ctx, "[CreateWorkloadHandler.Check] Unexpected error: %v", err)
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
func (h *CreateWorkloadHandler) Handle(ctx context.Context, raw interface{}) error {
	wrk, ok := raw.(*types.Workload)
	if !ok {
		return types.NewDetailedErr(types.ErrInvalidType, raw)
	}

	ctx, cancel := getReplayContext(ctx)
	defer cancel()

	// There hasn't been the exact workload metadata, so we must remove it.
	node, err := h.calcium.GetNode(ctx, wrk.Nodename)
	if err != nil {
		log.Errorf(ctx, "[CreateWorkloadHandler.Handle] Get node %s failed: %v", wrk.Nodename, err)
		return err
	}
	wrk.Engine = node.Engine

	if err := wrk.Remove(ctx, true); err != nil {
		if strings.HasPrefix(err.Error(), fmt.Sprintf("Error: No such container: %s", wrk.ID)) {
			log.Errorf(ctx, "[CreateWorkloadHandler.Handle] %s has been removed yet", wrk.ID)
			return nil
		}

		log.Errorf(ctx, "[CreateWorkloadHandler.Handle] Remove %s failed: %v", wrk.ID, err)
		return err
	}

	log.Warnf(ctx, "[CreateWorkloadHandler.Handle] %s has been removed", wrk.ID)

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
