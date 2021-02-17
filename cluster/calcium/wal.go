package calcium

import (
	"context"
	"encoding/json"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/wal"
)

const (
	eventCreateLambda = "create-lambda"
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
	if err := w.WAL.Open(context.Background(), w.config.WALFile, w.config.WALOpenTimeout); err != nil {
		return nil, err
	}

	w.registerHandlers()

	return w, nil
}

func (w *WAL) registerHandlers() {
	w.Register(newCreateLambdaHandler(w.calcium))
}

func (w *WAL) logCreateLambda(ctx context.Context, opts *types.DeployOptions) (wal.Commit, error) {
	return w.Log(ctx, eventCreateLambda, &types.ListWorkloadsOptions{
		Appname:    opts.Name,
		Entrypoint: opts.Entrypoint.Name,
		Labels:     map[string]string{labelLambdaID: opts.Labels[labelLambdaID]},
	})
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
		log.Errorf("[CreateLambdaHandler.Handle] Get workloads %s/%s/%v failed: %v",
			opts.Appname, opts.Entrypoint, opts.Labels, err)
		return err
	}

	if err := h.calcium.doRemoveWorkloadSync(context.Background(), workloadIDs); err != nil {
		log.Errorf("[CreateLambdaHandler.Handle] Remove lambda %v failed: %v", opts, err)
		return err
	}

	log.Infof("[CreateLambdaHandler.Handle] Lambda %v removed", opts)

	return nil
}

func (h *CreateLambdaHandler) getWorkloadIDs(opts *types.ListWorkloadsOptions) ([]string, error) {
	workloads, err := h.calcium.ListWorkloads(context.Background(), opts)
	if err != nil {
		return nil, err
	}

	workloadIDs := make([]string, len(workloads))
	for i, wrk := range workloads {
		workloadIDs[i] = wrk.ID
	}

	return workloadIDs, nil
}
