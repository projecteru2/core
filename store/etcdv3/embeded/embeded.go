package embeded

import (
	"testing"

	"github.com/coreos/etcd/clientv3"

	"github.com/coreos/etcd/integration"
)

var (
	t               *testing.T
	embeddedCluster *integration.ClusterV3
)

// NewCluster new a embeded cluster
func NewCluster() *clientv3.Client {
	t = &testing.T{}
	embeddedCluster = integration.NewClusterV3(t, &integration.ClusterConfig{Size: 1})
	return embeddedCluster.RandClient()
}

// TerminateCluster terminate embeded cluster
func TerminateCluster() {
	if embeddedCluster == nil || t == nil {
		return
	}
	embeddedCluster.Terminate(t)
}
