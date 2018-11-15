package simple

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBasicCredential(t *testing.T) {
	sc := NewBasicCredential("test", "password")
	m, err := sc.GetRequestMetadata(context.Background(), "a.com", "b.com")
	assert.NoError(t, err)
	v, ok := m["test"]
	assert.True(t, ok)
	assert.Equal(t, v, "password")
	assert.False(t, sc.RequireTransportSecurity())
}
