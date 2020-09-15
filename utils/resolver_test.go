package utils

import (
	"testing"

	"github.com/projecteru2/core/types"
	"github.com/stretchr/testify/assert"
)

func TestMakeTarget(t *testing.T) {
	addr := "eru://127.0.0.1:5001"
	authConfig := types.AuthConfig{Username: "abc", Password: "123"}
	r := MakeTarget(addr, authConfig)
	assert.Contains(t, r, "5001")
}
