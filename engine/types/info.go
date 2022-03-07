package types

// Info define info response
type Info struct {
	Type         string
	ID           string
	NCPU         int
	MemTotal     int64
	StorageTotal int64
}
