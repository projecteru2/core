package types

import (
	"io"

	enginetypes "github.com/projecteru2/core/engine/types"
)

// BuildMethod .
type BuildMethod int

const (
	// BuildFromSCM must be default method to avoid breaking
	BuildFromSCM BuildMethod = iota
	// BuildFromUnknown .
	BuildFromUnknown
	// BuildFromRaw .
	BuildFromRaw
	// BuildFromExist .
	BuildFromExist
)

// Builds is identical to enginetype.Builds
type Builds = enginetypes.Builds

// Build is identical to enginetype.Build
type Build = enginetypes.Build

// BuildOptions is options for building image
type BuildOptions struct {
	Name string
	User string
	UID  int
	Tags []string
	BuildMethod
	*Builds
	Tar     io.Reader
	ExistID string
}
