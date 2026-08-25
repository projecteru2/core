package types

import (
	"io"

	enginetypes "github.com/projecteru2/core/engine/types"
)

type BuildMethod int

const (
	// BuildFromSCM stays the zero value for wire compatibility.
	BuildFromSCM BuildMethod = iota
	BuildFromUnknown
	BuildFromRaw
	BuildFromExist
)

type Builds = enginetypes.Builds

type Build = enginetypes.Build

type BuildOptions struct {
	Name string
	User string
	UID  int
	Tags []string
	BuildMethod
	*Builds
	Tar      io.Reader
	ExistID  string
	Platform string
	// NodeFilter narrows the configured build node selection; it can never widen it.
	NodeFilter *NodeFilter
}
