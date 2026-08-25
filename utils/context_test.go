package utils

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/projecteru2/core/types"
)

func TestNewInheritCtxKeepsValuesAndDropsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.WithValue(t.Context(), types.TracingID, "trace"))
	cancel()

	got := NewInheritCtx(ctx)
	assert.NoError(t, got.Err())
	assert.Equal(t, "trace", got.Value(types.TracingID))
}
