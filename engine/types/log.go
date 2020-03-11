package types

// VirtualizationLogStreamOptions .
type VirtualizationLogStreamOptions struct {
	ID     string
	Tail   string
	Since  string
	Until  string
	Follow bool
	Stdout bool
	Stderr bool
}
