package utils

import "github.com/projecteru2/core/log"

// SentryGo runs f in a goroutine and reports a panic to Sentry before re-raising it.
func SentryGo(f func()) {
	go func() {
		defer log.SentryDefer()
		f()
	}()
}
