package types

import (
	"github.com/cockroachdb/errors"
)

var (
	ErrInsufficientCapacity  = errors.New("cannot alloc a plan, not enough nodes capacity")
	ErrInsufficientResource  = errors.New("cannot alloc a plan, not enough resource")
	ErrAlreadyFilled         = errors.New("cannot alloc a fill node plan, each node has enough workloads")
	ErrInvaildDeployStrategy = errors.New("deploy strategy not supported yet")

	ErrNodeExists = errors.New("node already exists")

	ErrInvaildNodeEndpoint  = errors.New("invalid node endpoint")
	ErrNodeNotEmpty         = errors.New("node not empty, still has workloads associated")
	ErrNodeNotExists        = errors.New("node not exists")
	ErrInvaildNodeStatusTTL = errors.New("invalid TTL for node status, should be > 0")
	ErrInvaildNodeFilter    = errors.New("node filter widens the configured selection")

	ErrLockKeyInvaild  = errors.New("lock key is invalid")
	ErrLockSessionDone = errors.New("lock session done")

	ErrInvaildEruIPAddress = errors.New("invalid eru address")

	ErrInvaildSCMType          = errors.New("scm type not supported yet")
	ErrDownloadArtifactsFailed = errors.New("download artifacts failed")

	ErrInvaildIPAddress     = errors.New("invalid IP address")
	ErrInvaildIPWithPort    = errors.New("invalid IP with port")
	ErrAllConnectionsFailed = errors.New("all connections failed")
	ErrUnexpectedRedirect   = errors.New("unexpected redirect")

	ErrInvaildMemory         = errors.New("invalid memory value")
	ErrNilEngine             = errors.New("engine is nil")
	ErrInvaildRefs           = errors.New("invalid image refs")
	ErrNoImage               = errors.New("no image")
	ErrNoImageUser           = errors.New("no image user")
	ErrInvaildRemoteDigest   = errors.New("got invalid digest")
	ErrInvaildEngineEndpoint = errors.New("unsupported engine endpoint")
	ErrEngineNotImplemented  = errors.New("not implemented")
	ErrInvalidEngineArgs     = errors.New("invalid engine args")

	ErrInvaildWorkloadStatus = errors.New("status has no appname / entrypoint / nodename")
	ErrInvaildWorkloadMeta   = errors.New("invalid workload meta")
	ErrInvaildWorkloadOps    = errors.New("invalid workload ops")
	ErrInvalidWorkloadName   = errors.New("invalid workload name")
	ErrWorkloadIgnored       = errors.New("ignore this workload")
	ErrWorkloadNotExists     = errors.New("workload not exists")
	ErrWorkloadRemoving      = errors.New("workload is being removed")

	ErrPodHasNodes = errors.New("pod has nodes")
	ErrPodNoNodes  = errors.New("pod has no nodes")
	ErrPodNotFound = errors.New("pod not found")

	ErrInvaildGRPCRequestMeta = errors.New("invalid grpc request meta")
	ErrInvaildGRPCPassword    = errors.New("invalid grpc password")
	ErrInvaildGRPCUsername    = errors.New("invalid grpc username")

	ErrNoBuildsInSpec              = errors.New("no builds in spec")
	ErrNoBuildSpec                 = errors.New("no build spec")
	ErrNoEntryInSpec               = errors.New("no entry in spec")
	ErrNoDeployOpts                = errors.New("no deploy options")
	ErrNoWorkloadIDs               = errors.New("no workload IDs given")
	ErrNoSCMSetting                = errors.New("scm not set")
	ErrRunAndWaitCountOneWithStdin = errors.New("count must be 1 if OpenStdin is true")
	ErrInvaildControlType          = errors.New("unknown control type")
	ErrInvaildBuildType            = errors.New("unknown build type")
	ErrInvalidGitURL               = errors.New("invalid git url format")
	ErrInvalidVolumeBind           = errors.New("invalid volume bind value")
	ErrEmptyNodeName               = errors.New("node name is empty")
	ErrEmptyNodeMap                = errors.New("node map is empty")
	ErrEmptyAppName                = errors.New("app name is empty")
	ErrEmptyPodName                = errors.New("pod name is empty")
	ErrEmptyImage                  = errors.New("image is empty")
	ErrEmptyCount                  = errors.New("count is 0")
	ErrEmptyWorkloadID             = errors.New("workload ID is empty")
	ErrEmptyEntrypointName         = errors.New("entrypoint name is empty")
	ErrUnderlineInEntrypointName   = errors.New("entrypoint name has '_' character")
	ErrEmptyRawEngineOp            = errors.New("raw engine op is empty")

	ErrKeyNotExists       = errors.New("key not exists")
	ErrKeyExists          = errors.New("key exists")
	ErrNoOps              = errors.New("no txn ops")
	ErrTxnConditionFailed = errors.New("etcd txn condition failed")
	ErrInvaildCount       = errors.New("bad `Count` value, entity count invalid")

	ErrInvaildWALEventType = errors.New("invalid WAL event type")
	ErrInvaildWALEvent     = errors.New("encode WAL event failed")
	ErrInvalidWALBucket    = errors.New("invalid WAL bucket")
	ErrInvalidWALDataType  = errors.New("invalid WAL data type")

	ErrInvaildDeployCount    = errors.New("invalid deploy count")
	ErrRollbackMapIsNotEmpty = errors.New("rollback map is not empty")
	ErrGetMostIdleNodeFailed = errors.New("get most idle node failed")

	ErrMessageChanClosed = errors.New("message chan closed")

	ErrNoFilesToSend = errors.New("no files to send")
	ErrNoFilesToCopy = errors.New("no files to copy")

	ErrMockError = errors.New("mock error")

	ErrMetricsTypeNotSupport = errors.New("metrics type not support")

	ErrConfigInvaild = errors.New("config invalid")
)
