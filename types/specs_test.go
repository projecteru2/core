package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEntrypointValidate(t *testing.T) {
	e := Entrypoint{Name: ""}
	assert.Error(t, e.Validate())
	e = Entrypoint{Name: "a_b"}
	assert.Error(t, e.Validate())
	e = Entrypoint{Name: "c"}
	assert.NoError(t, e.Validate())
}
