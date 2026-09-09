package types

type VirtualizationLogStreamOptions struct {
	ID     string
	Tail   string
	Since  string
	Until  string
	Follow bool
}
