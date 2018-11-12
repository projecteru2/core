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

	e := IsDetailedErr(dt)
	assert.Equal(t, e.Error(), errString)

	assert.True(t, strings.Contains(dt.Error(), detail))
}
