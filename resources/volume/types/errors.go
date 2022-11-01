package types

import "github.com/pkg/errors"

var (
	ErrInvalidCapacity      = errors.New("invalid capacity")
	ErrInvalidVolume        = errors.New("invalid volume")
	ErrInsufficientResource = errors.New("cannot alloc a plan, not enough resource")
	ErrInvalidStorage       = errors.New("invalid storage")
	ErrInvalidDisk          = errors.New("invalid disk")

	ErrNodeExists = errors.New("node already exists")
)
