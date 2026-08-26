package utils

import (
	"context"
	"time"
)

// KeepAlive calls refresh every interval until ctx ends or refresh fails, returning the failure.
func KeepAlive(ctx context.Context, interval time.Duration, refresh func(context.Context) error) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := refresh(ctx); err != nil {
				return err
			}
		}
	}
}
