package etcdv3

import (
	"github.com/panjf2000/ants/v2"

	"github.com/projecteru2/core/store/etcdv3/embedded"
	"github.com/projecteru2/core/store/etcdv3/meta"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

const (
	podInfoKey       = "/pod/info/%s" // /pod/info/{podname}
	serviceStatusKey = "/services/%s" // /service/{ipv4:port}

	nodeInfoKey      = "/node/%s"              // /node/{nodename}
	nodePodKey       = "/node/%s:pod/%s"       // /node/{podname}:pod/{nodename}
	nodeCaKey        = "/node/%s:ca"           // /node/{nodename}:ca
	nodeCertKey      = "/node/%s:cert"         // /node/{nodename}:cert
	nodeKeyKey       = "/node/%s:key"          // /node/{nodename}:key
	nodeStatusPrefix = "/status:node/"         // /status:node/{nodename} -> node status key
	nodeWorkloadsKey = "/node/%s:workloads/%s" // /node/{nodename}:workloads/{workloadID}

	workloadInfoKey          = "/workloads/%s" // /workloads/{workloadID}
	workloadDeployPrefix     = "/deploy"       // /deploy/{appname}/{entrypoint}/{nodename}/{workloadID}
	workloadStatusPrefix     = "/status"       // /status/{appname}/{entrypoint}/{nodename}/{workloadID} value -> something by agent
	workloadProcessingPrefix = "/processing"   // /processing/{appname}/{entrypoint}/{nodename}/{opsIdent} value -> count
)

// Mercury is the etcd backed store.
type Mercury struct {
	meta.KV
	config types.Config
	pool   *ants.PoolWithFunc
}

// New creates a Mercury on the given etcd cluster.
func New(config types.Config, embeddedETCD *embedded.Cluster) (m *Mercury, err error) {
	pool, err := utils.NewPool(config.MaxConcurrency)
	if err != nil {
		return nil, err
	}
	m = &Mercury{config: config, pool: pool}
	m.KV, err = meta.NewETCD(config.Etcd, embeddedETCD)
	return m, err
}
