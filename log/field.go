package log

import (
	"context"
)

// Fields carries key-value context for one log entry.
type Fields struct {
	kv []field
}

// WithField returns a copy of f tagged with key and value; f itself is left untouched.
func (f *Fields) WithField(key string, value any) *Fields {
	kv := make([]field, len(f.kv)+1)
	copy(kv, f.kv)
	kv[len(f.kv)] = field{key: key, value: value}
	return &Fields{kv: kv}
}

// Fatalf logs at fatal level and reports to Sentry.
func (f *Fields) Fatalf(ctx context.Context, err error, format string, args ...any) {
	fatalf(ctx, err, format, f.kv, args...)
}

func (f *Fields) Warnf(ctx context.Context, format string, args ...any) {
	warnf(ctx, format, f.kv, args...)
}

func (f *Fields) Warn(ctx context.Context, args ...any) {
	f.Warnf(ctx, formatArgs(args), args...)
}

func (f *Fields) Infof(ctx context.Context, format string, args ...any) {
	infof(ctx, format, f.kv, args...)
}

func (f *Fields) Info(ctx context.Context, args ...any) {
	f.Infof(ctx, formatArgs(args), args...)
}

func (f *Fields) Debugf(ctx context.Context, format string, args ...any) {
	debugf(ctx, format, f.kv, args...)
}

func (f *Fields) Debug(ctx context.Context, args ...any) {
	f.Debugf(ctx, formatArgs(args), args...)
}

// Errorf logs at error level and reports to Sentry.
func (f *Fields) Errorf(ctx context.Context, err error, format string, args ...any) {
	errorf(ctx, err, format, f.kv, args...)
}

// Error logs at error level and reports to Sentry.
func (f *Fields) Error(ctx context.Context, err error, args ...any) {
	f.Errorf(ctx, err, formatArgs(args), args...)
}

// WithField returns a Fields tagged with key and value.
func WithField(key string, value any) *Fields {
	return &Fields{kv: []field{{key: key, value: value}}}
}

// WithFunc tags the entry with the enclosing function name.
func WithFunc(fname string) *Fields {
	return WithField("func", fname)
}

type field struct {
	key   string
	value any
}
