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
	return fmt.Sprintf("Version:        %s\nGit hash:       %s\nBuilt:          %s\nGolang version: %s\nOS/Arch:        %s/%s\n",
		VERSION, REVISION, BUILTAT, runtime.Version(), runtime.GOOS, runtime.GOARCH)
}
