package calcium

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestListNetworks(t *testing.T) {
	initMockConfig()

	networks, err := mockc.ListNetworks(podname)
	assert.NoError(t, err)
	for _, network := range networks {
		t.Log(network.Name)
	}
}
