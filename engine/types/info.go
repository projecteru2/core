package types

// Info is a node's runtime capacity as reported by its engine.
type Info struct {
	Type         string
	ID           string
	NCPU         int
	MemTotal     int64
	StorageTotal int64
}
