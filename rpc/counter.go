package rpc

import (
	"context"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

type task struct {
	v       *Vibranium
	name    string
	verbose bool
	context context.Context
	cancel  context.CancelFunc
}

func (t *task) done() {
	if t.verbose {
		log.WithFunc("task.done").WithField("name", t.name).Debug(t.context, "task done")
	}
	t.cancel()
	t.v.counter.Done()
}

// Wait blocks until all in-flight tasks finish.
func (v *Vibranium) Wait() {
	v.counter.Wait()
}

func (v *Vibranium) newTask(ctx context.Context, name string, verbose bool) *task {
	if v.config.SentryDSN != "" {
		ctx = context.WithValue(ctx, types.TracingID, utils.RandomString(8))
	}
	ctx, cancel := context.WithCancel(ctx)
	if verbose {
		log.WithFunc("vibranium.newTask").WithField("name", name).Debug(ctx, "task added")
	}
	v.counter.Add(1)
	return &task{
		v:       v,
		name:    name,
		verbose: verbose,
		context: ctx,
		cancel:  cancel,
	}
}
