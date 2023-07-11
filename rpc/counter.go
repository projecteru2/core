package rpc

import (
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"

	"golang.org/x/net/context"
)

type task struct {
	v       *Vibranium
	name    string
	verbose bool
	context context.Context
	cancel  context.CancelFunc
}

// gRPC上全局的计数器
// 只有在任务数为0的时候才给停止
// 为啥会加在gRPC server上呢?
// 因为一个入口给一个最简单了...

// 增加一个任务, 在任务调用之前要调用一次.
// 否则任务不被追踪, 不保证任务能够正常完成.
func (v *Vibranium) newTask(ctx context.Context, name string, verbose bool) *task {
	if ctx != nil {
		ctx = context.WithValue(ctx, types.TracingID, utils.RandomString(8))
	}
	ctx, cancel := context.WithCancel(ctx)
	if verbose {
		log.WithFunc("vibranium.newTask").WithField("name", name).Debug(ctx, "task added")
	}
	v.counter.Add(1)
	v.TaskNum++
	return &task{
		v:       v,
		name:    name,
		verbose: verbose,
		context: ctx,
		cancel:  cancel,
	}
}

// 完成一个任务, 在任务执行完之后调用一次.
// 否则计数器用完不会为0, 你也别想退出这个进程了.
func (t *task) done() {
	if t.verbose {
		log.WithFunc("vibranium.done").WithField("name", t.name).Debug(t.context, "task done")
	}
	t.v.counter.Done()
	t.v.TaskNum--
}

// Wait for all tasks done
// 会在外面graceful之后调用.
// 不完成不给退出进程.
func (v *Vibranium) Wait() {
	v.counter.Wait()
}
