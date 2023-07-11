package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewPool(t *testing.T) {
	pool, err := NewPool(20)
	assert.NoError(t, err)
	assert.Equal(t, pool.Cap(), 20)
}
