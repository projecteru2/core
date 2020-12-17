package types

// LogStreamOptions log stream options
type LogStreamOptions struct {
	ID     string
	Tail   string
	Since  string
	Until  string
	Follow bool
}
