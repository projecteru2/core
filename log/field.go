package log

import (
	"context"

	"github.com/alphadose/haxmap"
)

// Fields carries key-value context for one log entry.
type Fields struct {
	kv *haxmap.Map[string, any]
}

func (f *Fields) WithField(key string, value any) *Fields {
	f.kv.Set(key, value)
	return f
}

// Fatalf logs at fatal level and reports to Sentry.
func (f *Fields) Fatalf(ctx context.Context, err error, format string, args ...any) {
	fatalf(ctx, err, format, f.kv, args...)
}

func (f *Fields) Warnf(ctx context.Context, format string, args ...any) {
	warnf(ctx, format, f.kv, args...)
}

func (f *Fields) Warn(ctx context.Context, args ...any) {
	f.Warnf(ctx, "%+v", args...)
}

func (f *Fields) Infof(ctx context.Context, format string, args ...any) {
	infof(ctx, format, f.kv, args...)
}

func (f *Fields) Info(ctx context.Context, args ...any) {
	f.Infof(ctx, "%+v", args...)
}

func (f *Fields) Debugf(ctx context.Context, format string, args ...any) {
	debugf(ctx, format, f.kv, args...)
}

func (f *Fields) Debug(ctx context.Context, args ...any) {
	f.Debugf(ctx, "%+v", args...)
}

// Errorf logs at error level and reports to Sentry.
func (f *Fields) Errorf(ctx context.Context, err error, format string, args ...any) {
	errorf(ctx, err, format, f.kv, args...)
}

// Error logs at error level and reports to Sentry.
func (f *Fields) Error(ctx context.Context, err error, args ...any) {
	f.Errorf(ctx, err, "%+v", args...)
}

// WithField returns a Fields tagged with key and value.
func WithField(key string, value any) *Fields {
	r := haxmap.New[string, any]()
	r.Set(key, value)
	return &Fields{
		kv: r,
	}
}

// WithFunc tags the entry with the enclosing function name.
func WithFunc(fname string) *Fields {
	return WithField("func", fname)
}
