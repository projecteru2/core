package rpc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCounter(t *testing.T) {
	v := Vibranium{}
	v.taskAdd("test", true)
	assert.Equal(t, v.TaskNum, 1)

	v.taskDone("test", true)
	assert.Equal(t, v.TaskNum, 0)

	v.Wait()
}
