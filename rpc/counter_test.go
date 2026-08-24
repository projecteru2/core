package rpc

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCounter(t *testing.T) {
	v := Vibranium{}
	task := v.newTask(t.Context(), "test", true)
	assert.EqualValues(t, 1, v.TaskNum.Load())

	task.done()
	assert.EqualValues(t, 0, v.TaskNum.Load())

	v.Wait()
}

func TestTaskNumIsRaceFree(t *testing.T) {
	v := &Vibranium{}
	var wg sync.WaitGroup
	for range 32 {
		wg.Go(func() {
			v.newTask(t.Context(), "test", false).done()
		})
	}
	wg.Wait()
	v.Wait()
	assert.EqualValues(t, 0, v.TaskNum.Load())
}
