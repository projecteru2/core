package common

import (
	"path/filepath"
	"strings"

	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

const (
	PodInfoKey       = "/pod/info/%s" // /pod/info/{podname}
	ServiceStatusKey = "/services/%s" // /service/{ipv4:port}

	NodeInfoKey      = "/node/%s"              // /node/{nodename}
	NodePodKey       = "/node/%s:pod/%s"       // /node/{podname}:pod/{nodename}
	NodeCaKey        = "/node/%s:ca"           // /node/{nodename}:ca
	NodeCertKey      = "/node/%s:cert"         // /node/{nodename}:cert
	NodeKeyKey       = "/node/%s:key"          // /node/{nodename}:key
	NodeStatusPrefix = "/status:node/"         // /status:node/{nodename} -> node status key
	NodeWorkloadsKey = "/node/%s:workloads/%s" // /node/{nodename}:workloads/{workloadID}

	WorkloadInfoKey          = "/workloads/%s" // /workloads/{workloadID}
	WorkloadDeployPrefix     = "/deploy"       // /deploy/{appname}/{entrypoint}/{nodename}/{workloadID}
	WorkloadStatusPrefix     = "/status"       // /status/{appname}/{entrypoint}/{nodename}/{workloadID} value -> something by agent
	WorkloadProcessingPrefix = "/processing"   // /processing/{appname}/{entrypoint}/{nodename}/{opsIdent} value -> count
)

func ParseStatusKey(key string) (string, string, string, string) {
	parts := strings.Split(key, "/")
	l := len(parts)
	return parts[l-4], parts[l-3], parts[l-2], parts[l-1]
}

func ParseNodename(key string) string {
	return utils.Tail(filepath.Dir(key))
}

func ProcessingKey(processing *types.Processing) string {
	return filepath.Join(WorkloadProcessingPrefix, processing.Appname, processing.Entryname, processing.Nodename, processing.Ident)
}
