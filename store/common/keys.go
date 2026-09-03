package common

import (
	"path/filepath"
	"strings"
	"time"

	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

const (
	// OrphanStatusTTL bounds a status reported before core recorded its entity, on every backend.
	OrphanStatusTTL = int64(time.Hour / time.Second)

	PodInfoKey       = "/pod/info/%s"
	ServiceStatusKey = "/services/%s"

	NodeInfoKey      = "/node/%s"
	NodePodKey       = "/node/%s:pod/%s"
	NodeStatusPrefix = "/status:node/"
	NodeWorkloadsKey = "/node/%s:workloads/%s"

	WorkloadInfoKey          = "/workloads/%s"
	WorkloadDeployPrefix     = "/deploy"
	WorkloadStatusPrefix     = "/status"
	WorkloadProcessingPrefix = "/processing"
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
