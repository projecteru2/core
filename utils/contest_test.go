package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInheritTracingInfo(t *testing.T) {
	assert.Nil(t, InheritTracingInfo(nil, nil))
}
