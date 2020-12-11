package version

import (
	"fmt"
	"runtime"
)

var (
	// NAME is app name
	NAME = "Eru-Core"
	// VERSION is app version
	VERSION = "unknown"
	// REVISION is app revision
	REVISION = "HEAD"
	// BUILTAT is app built info
	BUILTAT = "now"
)

// String show version thing
func String() string {
	version := ""
	version += fmt.Sprintf("Version:        %s\n", VERSION)
	version += fmt.Sprintf("Git hash:       %s\n", REVISION)
	version += fmt.Sprintf("Built:          %s\n", BUILTAT)
	version += fmt.Sprintf("Golang version: %s\n", runtime.Version())
	version += fmt.Sprintf("OS/Arch:        %s/%s\n", runtime.GOOS, runtime.GOARCH)
	return version
}
