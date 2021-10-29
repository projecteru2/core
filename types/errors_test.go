package types

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDetailedErr(t *testing.T) {
	errString := "test error"
	detail := "detail"
	err := errors.New(errString)
	dt := NewDetailedErr(err, detail)

	assert.True(t, errors.Is(dt, err))
	assert.True(t, strings.Contains(dt.Error(), detail))

	assert.True(t, errors.Is(NewDetailedErr(ErrBadCount, detail), ErrBadCount))
}
