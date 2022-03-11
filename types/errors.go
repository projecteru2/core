package types

import (
	"errors"
	"fmt"
)

// errors
var (
	ErrInsufficientCPU     = errors.New("cannot alloc a plan, not enough cpu")
	ErrInsufficientMEM     = errors.New("cannot alloc a plan, not enough memory")
	ErrInsufficientStorage = errors.New("cannot alloc a plan, not enough storage")
	ErrInsufficientVolume  = errors.New("cannot alloc a plan, not enough volume")
	ErrInsufficientCap     = errors.New("cannot alloc a each node plan, not enough capacity")
	ErrInsufficientRes     = errors.New("not enough resource")
	ErrInvalidRes          = errors.New("invalid resource")
	ErrInsufficientNodes   = errors.New("not enough nodes")
	ErrAlreadyFilled       = errors.New("Cannot alloc a fill node plan, each node has enough workloads")

	ErrZeroNodes = errors.New("no nodes provide to choose some")

	ErrNodeFormat    = errors.New("bad endpoint name")
	ErrNodeExist     = errors.New("node already exists")
	ErrNodeNotEmpty  = errors.New("node not empty, still has workloads associated")
	ErrNodeStatusTTL = errors.New("TTL for node status should be > 0")

	ErrKeyIsDir    = errors.New("key is a directory")
	ErrKeyIsNotDir = errors.New("key is not a directory")
	ErrKeyIsEmpty  = errors.New("key is empty")

	ErrBadWorkloadID     = errors.New("workload ID must be length of 64")
	ErrBadDeployStrategy = errors.New("deploy method not support yet")
	ErrBadIPAddress      = errors.New("bad IP address")
	ErrBadSCMType        = errors.New("unknown SCM type")
	ErrBadMemory         = errors.New("bad `Memory` value")
	ErrBadCPU            = errors.New("bad `CPU` value")
	ErrBadStorage        = errors.New("bad `Storage` value")
	ErrBadVolume         = errors.New("bad `Volume` value")
	ErrBadCount          = errors.New("bad `Count` value")

	ErrBadWorkloadStatus = errors.New("status has no appname / entrypoint/ nodename")

	ErrPodHasNodes = errors.New("pod has nodes")
	ErrPodNoNodes  = errors.New("pod has no nodes")
	ErrPodNotFound = errors.New("pod not found")

	ErrCannotGetEngine = errors.New("cannot get engine")
	ErrNilEngine       = errors.New("engine is nil")

	ErrBadMeta         = errors.New("bad meta")
	ErrInvaildPassword = errors.New("invaild password")
	ErrInvaildUsername = errors.New("invaild username")
	ErrNotFitLabels    = errors.New("not fit labels")

	ErrBadRefs                     = errors.New("invalid image refs")
	ErrNoImage                     = errors.New("no image")
	ErrNoImageUser                 = errors.New("no image user")
	ErrNoBuildPod                  = errors.New("No build pod set in config")
	ErrNoBuildsInSpec              = errors.New("No builds in spec")
	ErrNoBuildSpec                 = errors.New("No build spec")
	ErrNoEntryInSpec               = errors.New("No entry in spec")
	ErrNoDeployOpts                = errors.New("No deploy options")
	ErrNoWorkloadIDs               = errors.New("No workload ids given")
	ErrRunAndWaitCountOneWithStdin = errors.New("Count must be 1 if OpenStdin is true")
	ErrUnknownControlType          = errors.New("Unknown control type")
	ErrNoRemoteDigest              = errors.New("got no digest")

	ErrNoETCD             = errors.New("ETCD must be set")
	ErrKeyNotExists       = errors.New("Key not exists")
	ErrKeyExists          = errors.New("Key exists")
	ErrNoOps              = errors.New("No txn ops")
	ErrTxnConditionFailed = errors.New("ETCD Txn condition failed")

	ErrNotSupport = errors.New("Not Support")
	ErrSCMNotSet  = errors.New("SCM not set")

	ErrInvalidBind    = errors.New("invalid bind value")
	ErrIgnoreWorkload = errors.New("ignore this workload")

	ErrInvalidGitURL       = errors.New("invalid git url format")
	ErrInvalidWorkloadName = errors.New("invalid workload name")

	ErrEngineNotImplemented = errors.New("not implemented")

	ErrNodeNotExists     = errors.New("node not exists")
	ErrWorkloadNotExists = errors.New("workload not exists")
	ErrEntityNotExists   = errors.New("entity not exists")

	ErrUnregisteredWALEventType = errors.New("unregistered WAL event type")
	ErrBadWALEvent              = errors.New("bad WAL event type")
	ErrInvalidWALBucket         = errors.New("invalid WAL bucket")
	ErrInvalidType              = errors.New("invalid type")
	ErrLockSessionDone          = errors.New("lock session done")

	ErrRollbackMapIsNotEmpty = errors.New("rollback map is not empty")

	ErrMessageChanClosed = errors.New("message chan closed")

	ErrEmptyNodeName      = errors.New("node name is empty")
	ErrEmptyAppName       = errors.New("app name is empty")
	ErrEmptyPodName       = errors.New("pod name is empty")
	ErrEmptyNodeEndpoint  = errors.New("node endpoint is empty")
	ErrEmptyImage         = errors.New("image is empty")
	ErrEmptyCount         = errors.New("count is 0")
	ErrEmptyWorkloadID    = errors.New("workload id is empty")
	ErrNegativeCPU        = errors.New("cpu is negative")
	ErrNegativeShare      = errors.New("share is negative")
	ErrNegativeMemory     = errors.New("memory is negative")
	ErrNegativeNUMAMemory = errors.New("numa memory is negative")
	ErrNegativeStorage    = errors.New("storage is negative")
	ErrNegativeVolumeSize = errors.New("volume size is negative")

	ErrEmptyEntrypointName       = errors.New("entrypoint name is empty")
	ErrUnderlineInEntrypointName = errors.New("entrypoint name has '_' character")

	ErrNoFilesToSend = errors.New("no files to send")
	ErrNoFilesToCopy = errors.New("no files to copy")
)

type detailedErr struct {
	err     error
	details interface{}
}

// Error .
func (d detailedErr) Error() string {
	return fmt.Sprintf("%s: %v", d.err, d.details)
}

// Unwrap .
func (d detailedErr) Unwrap() error {
	return d.err
}

// NewDetailedErr returns an error with details
func NewDetailedErr(err error, details interface{}) error {
	return detailedErr{err: err, details: details}
}
