package types

import "time"

// ServiceStatus Interval indicates when the expected next push shall reach before
type ServiceStatus struct {
	Addresses []string
	Interval  time.Duration
}
