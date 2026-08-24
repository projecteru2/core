package store

type Op struct {
	WithoutEngine bool
}

type Option func(*Op)

func NewOp(opts ...Option) *Op {
	op := &Op{}
	for _, opt := range opts {
		opt(op)
	}
	return op
}

func WithoutEngineOption() Option {
	return func(op *Op) {
		op.WithoutEngine = true
	}
}
