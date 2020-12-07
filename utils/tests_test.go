package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerateNodes(t *testing.T) {
	ns := GenerateScheduleInfos(1, 10, 1000, 1000, 100)
	assert.Len(t, ns, 1)
}
