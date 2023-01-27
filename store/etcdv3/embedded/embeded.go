package embedded

import (
	"flag"
	"os"
	"testing"

	"go.etcd.io/etcd/client/v3/namespace"
	"go.etcd.io/etcd/tests/v3/integration"
)

var clusters map[string]*integration.ClusterV3 = map[string]*integration.ClusterV3{}

// NewCluster new a embedded cluster
func NewCluster(t *testing.T, prefix string) *integration.ClusterV3 {
	cluster := clusters[t.Name()]
	if cluster == nil {
		os.Args = []string{"test.short=false"}
		testing.Init()
		flag.Parse()
		integration.BeforeTestExternal(t)
		cluster = integration.NewClusterV3(t, &integration.ClusterConfig{Size: 1})
		t.Cleanup(func() {
			cluster.Terminate(t)
			delete(clusters, t.Name())
		})
		cliv3 := cluster.RandClient()
		cliv3.KV = namespace.NewKV(cliv3.KV, prefix)
		cliv3.Watcher = namespace.NewWatcher(cliv3.Watcher, prefix)
		cliv3.Lease = namespace.NewLease(cliv3.Lease, prefix)
		clusters[t.Name()] = cluster
	}
	return cluster
}
