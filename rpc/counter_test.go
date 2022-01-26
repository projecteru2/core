package rpc

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCounter(t *testing.T) {
	v := Vibranium{}
	task := v.newTask(context.TODO(), "test", true)
	assert.Equal(t, v.TaskNum, 1)

	task.done()
	assert.Equal(t, v.TaskNum, 0)

	v.Wait()
}
