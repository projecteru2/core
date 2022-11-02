package types

import (
	"github.com/cockroachdb/errors"
)

var (
	// Scheduler
	ErrInsufficientCapacity  = errors.New("cannot alloc a plan, not enough nodes capacity")
	ErrInsufficientResource  = errors.New("cannot alloc a plan, not enough resource")
	ErrAlreadyFilled         = errors.New("cannot alloc a fill node plan, each node has enough workloads")
	ErrInvaildDeployStrategy = errors.New("deploy method not support yet")

	// Resources
	ErrNodeExists = errors.New("node already exists")

	// Node
	ErrInvaildNodeEndpoint  = errors.New("invaild node endpoint")
	ErrNodeNotEmpty         = errors.New("node not empty, still has workloads associated")
	ErrNodeNotExists        = errors.New("node not exists")
	ErrInvaildNodeStatusTTL = errors.New("invaild TTL for node status, should be > 0")

	// Lock
	ErrLockKeyInvaild  = errors.New("lock key is invaild")
	ErrLockSessionDone = errors.New("lock session done")

	// Client
	ErrInvaildEruIPAddress = errors.New("invalid eru address")

	// SCM
	ErrInvaildSCMType          = errors.New("SCM type not support yet")
	ErrDownloadArtifactsFailed = errors.New("download artifacts failed")

	// General
	ErrInvaildIPAddress     = errors.New("invalid IP address")
	ErrInvaildIPWithPort    = errors.New("invaild IP with port")
	ErrICMPLost             = errors.New("icmp packets lost")
	ErrAllConnectionsFailed = errors.New("all connections failed")
	ErrUnexpectedRedirect   = errors.New("unexpected redirect")

	// Engine
	ErrInvaildMemory         = errors.New("invaild `Memory` value setting")
	ErrNilEngine             = errors.New("engine is nil")
	ErrInvaildRefs           = errors.New("invalid image refs")
	ErrNoImage               = errors.New("no image")
	ErrNoImageUser           = errors.New("no image user")
	ErrInvaildRemoteDigest   = errors.New("got invaild digest")
	ErrInvaildEngineEndpoint = errors.New("not Support endpoint")
	ErrEngineNotImplemented  = errors.New("not implemented")
	ErrInvalidEngineArgs     = errors.New("invalid engine args")

	// Workload
	ErrInvaildWorkloadStatus = errors.New("status has no appname / entrypoint / nodename")
	ErrInvaildWorkloadMeta   = errors.New("invaild workload meta")
	ErrInvaildWorkloadOps    = errors.New("invaild workload ops")
	ErrInvalidWorkloadName   = errors.New("invalid workload name")
	ErrWorkloadIgnored       = errors.New("ignore this workload")
	ErrWorkloadNotExists     = errors.New("workload not exists")

	// Pod
	ErrPodHasNodes = errors.New("pod has nodes")
	ErrPodNoNodes  = errors.New("pod has no nodes")
	ErrPodNotFound = errors.New("pod not found")

	// GRPC
	ErrInvaildGRPCRequestMeta = errors.New("invaild grpc request meta")
	ErrInvaildGRPCPassword    = errors.New("invaild grpc password")
	ErrInvaildGRPCUsername    = errors.New("invaild grpc username")

	// Opts Validation
	ErrNoBuildPod                  = errors.New("no build pod set in config")
	ErrNoBuildsInSpec              = errors.New("no builds in spec")
	ErrNoBuildSpec                 = errors.New("no build spec")
	ErrNoEntryInSpec               = errors.New("no entry in spec")
	ErrNoDeployOpts                = errors.New("no deploy options")
	ErrNoWorkloadIDs               = errors.New("no workload ids given")
	ErrNoSCMSetting                = errors.New("SCM not set")
	ErrRunAndWaitCountOneWithStdin = errors.New("count must be 1 if OpenStdin is true")
	ErrInvaildControlType          = errors.New("unknown control type")
	ErrInvaildBuildType            = errors.New("unknown build type")
	ErrInvalidGitURL               = errors.New("invalid git url format")
	ErrInvalidVolumeBind           = errors.New("invalid volume bind value")
	ErrEmptyNodeName               = errors.New("node name is empty")
	ErrEmptyAppName                = errors.New("app name is empty")
	ErrEmptyPodName                = errors.New("pod name is empty")
	ErrEmptyImage                  = errors.New("image is empty")
	ErrEmptyCount                  = errors.New("count is 0")
	ErrEmptyWorkloadID             = errors.New("workload id is empty")
	ErrEmptyEntrypointName         = errors.New("entrypoint name is empty")
	ErrUnderlineInEntrypointName   = errors.New("entrypoint name has '_' character")

	// Store
	ErrKeyNotExists       = errors.New("key not exists")
	ErrKeyExists          = errors.New("key exists")
	ErrNoOps              = errors.New("no txn ops")
	ErrTxnConditionFailed = errors.New("ETCD Txn condition failed")
	ErrInvaildCount       = errors.New("bad `Count` value, entity count invaild") // store key-value count not same

	// WAL
	ErrInvaildWALEventType = errors.New("invaild WAL event type")
	ErrInvaildWALEvent     = errors.New("invaild WAL event type")
	ErrInvalidWALBucket    = errors.New("invalid WAL bucket")
	ErrInvalidWALDataType  = errors.New("invalid WAL data type")

	// Create
	ErrInvaildDeployCount    = errors.New("invaild deploy count")
	ErrRollbackMapIsNotEmpty = errors.New("rollback map is not empty")
	ErrGetMostIdleNodeFailed = errors.New("get most idle node failed")

	// Selfmon
	ErrMessageChanClosed = errors.New("message chan closed")

	// File
	ErrNoFilesToSend = errors.New("no files to send")
	ErrNoFilesToCopy = errors.New("no files to copy")

	// Core
	ErrInvaildCoreEndpointType = errors.New("invaild Core endpoint type")

	// Test
	ErrMockError = errors.New("mock error")

	// Metrics
	ErrMetricsTypeNotSupport = errors.New("metrics type not support")
)
