package types

import "time"

type ServiceStatus struct {
	Addresses []string
	Interval  time.Duration
}
