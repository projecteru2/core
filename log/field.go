package log

import (
	"context"

	"github.com/alphadose/haxmap"
)

// Fields is a wrapper for zerolog.Entry
// we need to insert some sentry captures here
type Fields struct {
	kv *haxmap.Map[string, any]
}

// WithFunc is short for WithField
func WithFunc(fname string) *Fields {
	return WithField("func", fname)
}

// WithField add kv into log entry
func WithField(key string, value any) *Fields {
	r := haxmap.New[string, any]()
	r.Set(key, value)
	return &Fields{
		kv: r,
	}
}

// WithField .
func (f *Fields) WithField(key string, value any) *Fields {
	f.kv.Set(key, value)
	return f
}

// Fatalf forwards to sentry
func (f Fields) Fatalf(ctx context.Context, err error, format string, args ...any) {
	fatalf(ctx, err, format, f.kv, args...)
}

// Warnf is Warnf
func (f Fields) Warnf(ctx context.Context, format string, args ...any) {
	warnf(ctx, format, f.kv, args...)
}

// Warn is Warn
func (f Fields) Warn(ctx context.Context, args ...any) {
	f.Warnf(ctx, "%+v", args...)
}

// Infof is Infof
func (f Fields) Infof(ctx context.Context, format string, args ...any) {
	infof(ctx, format, f.kv, args...)
}

// Info is Info
func (f Fields) Info(ctx context.Context, args ...any) {
	f.Infof(ctx, "%+v", args...)
}

// Debugf is Debugf
func (f Fields) Debugf(ctx context.Context, format string, args ...any) {
	debugf(ctx, format, f.kv, args...)
}

// Debug is Debug
func (f Fields) Debug(ctx context.Context, args ...any) {
	f.Debugf(ctx, "%+v", args...)
}

// Errorf forwards to sentry
func (f Fields) Errorf(ctx context.Context, err error, format string, args ...any) {
	errorf(ctx, err, format, f.kv, args...)
}

// Error forwards to sentry
func (f Fields) Error(ctx context.Context, err error, args ...any) {
	f.Errorf(ctx, err, "%+v", args...)
}
