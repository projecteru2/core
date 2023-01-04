package types

import "github.com/cockroachdb/errors"

var (
	ErrInvalidCapacity   = errors.New("invalid resource capacity")
	ErrInvalidCPUMap     = errors.New("invalid cpu map")
	ErrInvalidNUMACPU    = errors.New("invalid numa cpu")
	ErrInvalidNUMAMemory = errors.New("invalid numa memory")
	ErrInvalidMemory     = errors.New("invalid memory")
	ErrInvalidCPU        = errors.New("invalid cpu")
)
