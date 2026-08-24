package types

import "time"

type ServiceStatus struct {
	Addresses []string
	Interval  time.Duration // deadline for the next expected push
}
