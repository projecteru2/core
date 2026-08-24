package virt

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/projecteru2/core/types"
)

func TestUnimplementedImageOpsReportAnError(t *testing.T) {
	v := &Virt{}

	assert.ErrorIs(t, v.ImagesPrune(t.Context()), types.ErrEngineNotImplemented)

	rc, err := v.ImageBuild(t.Context(), nil, nil, "")
	assert.Nil(t, rc)
	assert.ErrorIs(t, err, types.ErrEngineNotImplemented)

	reclaimed, err := v.ImageBuildCachePrune(t.Context(), true)
	assert.Zero(t, reclaimed)
	assert.ErrorIs(t, err, types.ErrEngineNotImplemented)
}
