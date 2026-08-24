package version

import (
	"fmt"
	"runtime"
)

var (
	NAME    = "Eru-Core"
	VERSION = "unknown"
	// REVISION is the git hash, injected by goreleaser ldflags.
	REVISION = "HEAD"
	BUILTAT  = "now"
)

// String returns the formatted build information.
func String() string {
	version := ""
	version += fmt.Sprintf("Version:        %s\n", VERSION)
	version += fmt.Sprintf("Git hash:       %s\n", REVISION)
	version += fmt.Sprintf("Built:          %s\n", BUILTAT)
	version += fmt.Sprintf("Golang version: %s\n", runtime.Version())
	version += fmt.Sprintf("OS/Arch:        %s/%s\n", runtime.GOOS, runtime.GOARCH)
	return version
}
