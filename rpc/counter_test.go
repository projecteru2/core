package rpc

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCounter(t *testing.T) {
	v := Vibranium{}
	v.taskAdd(context.TODO(), "test", true)
	assert.Equal(t, v.TaskNum, 1)

	v.taskDone(context.TODO(), "test", true)
	assert.Equal(t, v.TaskNum, 0)

	v.Wait()
}
