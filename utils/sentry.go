package utils

import "github.com/projecteru2/core/log"

// SentryGo wraps goroutine spawn to capture panic
func SentryGo(f func()) {
	go func() {
		defer log.SentryDefer()
		f()
	}()
}
