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
		defer SentryDefer()
		f()
	}()
}

// SentryDefer .
func SentryDefer() {
	if sentry.CurrentHub().Client() != nil {
		return
	}
	defer sentry.Flush(2 * time.Second)
	if r := recover(); r != nil {
		sentry.CaptureMessage(fmt.Sprintf("%+v: %s", r, debug.Stack()))
		panic(r)
	}
}
