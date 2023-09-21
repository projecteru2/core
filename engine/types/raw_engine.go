package types

type RawEngineOptions struct {
	ID     string
	Op     string
	Params []byte
}

type RawEngineResult struct {
	ID   string
	Data []byte
}
