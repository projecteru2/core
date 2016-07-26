package version

import (
	"fmt"
	"runtime"
	"time"
)

var (
	NAME     = "Eru-Core"
	VERSION  = "1.0.0"
	REVISION = "HEAD"
)

func VersionString() string {
	build := time.Now()
	// 暂时隐藏吧, 也不知道按啥version发布的好
	// version := fmt.Sprintf("Version:        %s\n", VERSION)
	version := fmt.Sprintf("Git hash:       %s\n", REVISION)
	version += fmt.Sprintf("Built:          %s\n", build.Format(time.RFC1123Z))
	version += fmt.Sprintf("Golang version: %s\n", runtime.Version())
	version += fmt.Sprintf("OS/Arch:        %s/%s\n", runtime.GOOS, runtime.GOARCH)
	return version
}
