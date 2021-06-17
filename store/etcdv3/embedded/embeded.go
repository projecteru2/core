package embedded

import (
	"testing"

	"go.etcd.io/etcd/tests/v3/integration"
)

// EmbededETCD .
type EmbededETCD struct {
	t       *testing.T
	Cluster *integration.ClusterV3
}

// TerminateCluster terminate embedded cluster
func (e *EmbededETCD) TerminateCluster() {
	if e.Cluster == nil || e.t == nil {
		return
	}
	e.Cluster.Terminate(e.t)
}

// NewCluster new a embedded cluster
func NewCluster() *EmbededETCD {
	t := &testing.T{}
	integration.BeforeTestExternal(t)
	Cluster := integration.NewClusterV3(t, &integration.ClusterConfig{Size: 1})
	return &EmbededETCD{t, Cluster}
}
