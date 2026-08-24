package rpc

import (
	"sync"
	"testing"
)

func TestWaitReturnsAfterEveryTaskIsDone(t *testing.T) {
	v := &Vibranium{}
	var wg sync.WaitGroup
	for range 32 {
		wg.Go(func() {
			v.newTask(t.Context(), "test", false).done()
		})
	}
	wg.Wait()
	v.Wait()
}
