package utils

import (
	"fmt"
	"runtime/debug"
	"time"

	"github.com/getsentry/sentry-go"
)

// SentryGo wraps goroutine spawn to capture panic
func SentryGo(f func()) {
	go func() {
		defer func() {
			defer sentry.Flush(time.Second)
			if err := recover(); err != nil {
				sentry.CaptureMessage(fmt.Sprintf("%+v: %s", err, debug.Stack()))
				panic(err)
			}
		}()
		f()
	}()
}
