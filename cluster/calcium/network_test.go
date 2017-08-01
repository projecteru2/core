package calcium

import (
	"testing"
)

func TestListNetworks(t *testing.T) {
	initMockConfig()

	networks, err := mockc.ListNetworks(podname)
	if err != nil {
		t.Error(err)
		return
	}
	for _, network := range networks {
		t.Log(network.Name)
	}
}
