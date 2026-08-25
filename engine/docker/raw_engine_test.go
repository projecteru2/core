package docker

import (
	"testing"

	"github.com/stretchr/testify/assert"

	coretypes "github.com/projecteru2/core/types"
)

func TestRawEngineReportsAnError(t *testing.T) {
	res, err := (&Engine{}).RawEngine(t.Context(), nil)
	assert.Nil(t, res)
	assert.ErrorIs(t, err, coretypes.ErrEngineNotImplemented)
}
