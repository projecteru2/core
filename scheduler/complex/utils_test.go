package complexscheduler

import (
	"testing"

	"github.com/projecteru2/core/types"
	"github.com/stretchr/testify/assert"
)

func TestShuffle(t *testing.T) {
	ns := []types.NodeInfo{
		types.NodeInfo{
			Name: "n1",
		},
		types.NodeInfo{
			Name: "n2",
		},
		types.NodeInfo{
			Name: "n3",
		},
	}
	n2 := shuffle(ns)
	assert.Len(t, ns, len(n2))
}
