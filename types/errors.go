package types

import (
	"errors"
	"fmt"
)

// errors
var (
	ErrInsufficientCPU   = errors.New("cannot alloc a plan, not enough cpu")
	ErrInsufficientMEM   = errors.New("cannot alloc a plan, not enough memory")
	ErrInsufficientCap   = errors.New("cannot alloc a each node plan, not enough capacity")
	ErrInsufficientRes   = errors.New("not enough resource")
	ErrInsufficientNodes = errors.New("not enough nodes")

	ErrNegativeMemory = errors.New("memory must be positive")
	ErrNegativeQuota  = errors.New("quota must be positive")

	ErrZeroNodes = errors.New("no nodes provide to choose some")

	ErrNodeFormat = errors.New("bad endpoint name")
	ErrNodeExist  = errors.New("node already exists")

	ErrKeyIsDir    = errors.New("key is a directory")
	ErrKeyIsNotDir = errors.New("key is not a directory")
	ErrKeyIsEmpty  = errors.New("key is empty")

	ErrBadContainerID  = errors.New("container ID must be length of 64")
	ErrBadDeployMethod = errors.New("deploy method not support yet")
	ErrBadFaver        = errors.New("favor should be either CPU or MEM")
	ErrBadIPAddress    = errors.New("bad IP address")
	ErrBadPodType      = errors.New("pod type not support yet")
	ErrBadSCMType      = errors.New("unkonwn SCM type")
	ErrBadMemory       = errors.New("bad `Memory` value")
	ErrBadCPU          = errors.New("bad `CPU` value")
	ErrBadCount        = errors.New("bad `Count` value")

	ErrPodHasNodes = errors.New("pod has nodes")
	ErrPodNoNodes  = errors.New("pod has no nodes")

	ErrCannotGetEngine = errors.New("cannot get engine")
	ErrNilEngine       = errors.New("engine is nil")

	ErrBadMeta         = errors.New("bad meta")
	ErrInvaildPassword = errors.New("invaild password")
	ErrInvaildUsername = errors.New("invaild username")
	ErrNotFitLabels    = errors.New("not fit labels")
)

// NewDetailedErr returns an error with details
func NewDetailedErr(err error, details interface{}) error {
	return &detailedErr{
		details: fmt.Sprintf("%v", details),
		err:     err,
	}
}

// IsDetailedErr returns the error's type if the error
// is a detailedErr, otherwise return it back
func IsDetailedErr(err error) error {
	if e, ok := err.(*detailedErr); ok {
		return e.err
	}
	return err
}

// detailedErr conains details
type detailedErr struct {
	details string
	err     error
}

func (de *detailedErr) Error() string {
	if de.details != "" {
		return fmt.Sprintf("%s: %s", de.err.Error(), de.details)
	}
	return de.err.Error()
}
