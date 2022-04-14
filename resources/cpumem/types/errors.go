package types

import "errors"

var (
	ErrInvalidCapacity   = errors.New("invalid resource capacity")
	ErrInvalidCPUMap     = errors.New("invalid cpu map")
	ErrInvalidNUMA       = errors.New("invalid numa")
	ErrInvalidNUMAMemory = errors.New("invalid numa memory")
	ErrInvalidMemory     = errors.New("invalid memory")
	ErrInvalidCPU        = errors.New("invalid cpu")

	ErrInsufficientCPU      = errors.New("cannot alloc a plan, not enough cpu")
	ErrInsufficientMem      = errors.New("cannot alloc a plan, not enough memory")
	ErrInsufficientResource = errors.New("cannot alloc a plan, not enough resource")

	ErrNodeExists = errors.New("node already exists")
	ErrNoNode     = errors.New("no node")
)
