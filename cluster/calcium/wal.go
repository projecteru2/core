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

func (w *WAL) logCreateLambda(opts *types.DeployOptions) (wal.Commit, error) {
	return w.Log(eventCreateLambda, &types.ListWorkloadsOptions{
		Appname:    opts.Name,
		Entrypoint: opts.Entrypoint.Name,
		Labels:     map[string]string{labelLambdaID: opts.Labels[labelLambdaID]},
	})
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
func (h *CreateWorkloadHandler) Check(raw interface{}) (bool, error) {
	wrk, ok := raw.(*types.Workload)
	if !ok {
		return false, types.NewDetailedErr(types.ErrInvalidType, raw)
	}

	ctx, cancel := getReplayContext(context.Background())
	defer cancel()

	_, err := h.calcium.GetWorkload(ctx, wrk.ID)
	switch {
	// there has been an exact workload metadata.
	case err == nil:
		log.Infof(nil, "[CreateWorkloadHandler.Check] Workload %s is availalbe", wrk.ID)
		return false, nil

	case strings.HasPrefix(err.Error(), types.ErrBadCount.Error()):
		log.Errorf(nil, "[CreateWorkloadHandler.Check] No such workload: %v", wrk.ID)
		return true, nil

	default:
		log.Errorf(nil, "[CreateWorkloadHandler.Check] Unexpected error: %v", err)
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
func (h *CreateWorkloadHandler) Handle(raw interface{}) error {
	wrk, ok := raw.(*types.Workload)
	if !ok {
		return types.NewDetailedErr(types.ErrInvalidType, raw)
	}

	ctx, cancel := getReplayContext(context.Background())
	defer cancel()

	// There hasn't been the exact workload metadata, so we must remove it.
	node, err := h.calcium.GetNode(ctx, wrk.Nodename)
	if err != nil {
		log.Errorf(nil, "[CreateWorkloadHandler.Handle] Get node %s failed: %v", wrk.Nodename, err)
		return err
	}
	wrk.Engine = node.Engine

	if err := wrk.Remove(ctx, true); err != nil {
		if strings.HasPrefix(err.Error(), fmt.Sprintf("Error: No such container: %s", wrk.ID)) {
			log.Errorf(nil, "[CreateWorkloadHandler.Handle] %s has been removed yet", wrk.ID)
			return nil
		}

		log.Errorf(nil, "[CreateWorkloadHandler.Handle] Remove %s failed: %v", wrk.ID, err)
		return err
	}

	log.Warnf(nil, "[CreateWorkloadHandler.Handle] %s has been removed", wrk.ID)

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
func (h *CreateLambdaHandler) Check(interface{}) (bool, error) {
	return true, nil
}

// Encode .
func (h *CreateLambdaHandler) Encode(raw interface{}) ([]byte, error) {
	opts, ok := raw.(*types.ListWorkloadsOptions)
	if !ok {
		return nil, types.NewDetailedErr(types.ErrInvalidType, raw)
	}
	return json.Marshal(opts)
}

// Decode .
func (h *CreateLambdaHandler) Decode(bs []byte) (interface{}, error) {
	opts := &types.ListWorkloadsOptions{}
	err := json.Unmarshal(bs, opts)
	return opts, err
}

// Handle .
func (h *CreateLambdaHandler) Handle(raw interface{}) error {
	opts, ok := raw.(*types.ListWorkloadsOptions)
	if !ok {
		return types.NewDetailedErr(types.ErrInvalidType, raw)
	}

	workloadIDs, err := h.getWorkloadIDs(opts)
	if err != nil {
		log.Errorf(nil, "[CreateLambdaHandler.Handle] Get workloads %s/%s/%v failed: %v",
			opts.Appname, opts.Entrypoint, opts.Labels, err)
		return err
	}

	ctx, cancel := getReplayContext(context.Background())
	defer cancel()

	if err := h.calcium.doRemoveWorkloadSync(ctx, workloadIDs); err != nil {
		log.Errorf(ctx, "[CreateLambdaHandler.Handle] Remove lambda %v failed: %v", opts, err)
		return err
	}

	log.Infof(nil, "[CreateLambdaHandler.Handle] Lambda %v removed", opts)

	return nil
}

func (h *CreateLambdaHandler) getWorkloadIDs(opts *types.ListWorkloadsOptions) ([]string, error) {
	ctx, cancel := getReplayContext(context.Background())
	defer cancel()

	workloads, err := h.calcium.ListWorkloads(ctx, opts)
	if err != nil {
		return nil, err
	}

	workloadIDs := make([]string, len(workloads))
	for i, wrk := range workloads {
		workloadIDs[i] = wrk.ID
	}

	return workloadIDs, nil
}

func getReplayContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, time.Second*32)
}
