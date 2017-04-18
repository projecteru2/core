package rpc

import (
	log "github.com/Sirupsen/logrus"
)

// gRPC上全局的计数器
// 只有在任务数为0的时候才给停止
// 为啥会加在gRPC server上呢?
// 因为一个入口给一个最简单了...

// 增加一个任务, 在任务调用之前要调用一次.
// 否则任务不被追踪, 不保证任务能够正常完成.
func (v *virbranium) taskAdd(name string) {
	log.Infof("task [%s] added", name)
	v.counter.Add(1)
}

// 完成一个任务, 在任务执行完之后调用一次.
// 否则计数器用完不会为0, 你也别想退出这个进程了.
func (v *virbranium) taskDone(name string) {
	log.Infof("task [%s] done", name)
	v.counter.Done()
}

// 会在外面graceful之后调用.
// 不完成不给退出进程.
func (v *virbranium) Wait() {
	v.counter.Wait()
}
