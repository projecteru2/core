package log

import "context"

// Fields is a wrapper for zerolog.Entry
// we need to insert some sentry captures here
type Fields struct {
	kv map[string]interface{}
}

// WithField add kv into log entry
func WithField(key string, value interface{}) *Fields {
	return &Fields{
		kv: map[string]interface{}{key: value},
	}
}

// WithField .
func (f *Fields) WithField(key string, value interface{}) *Fields {
	f.kv[key] = value
	return f
}

// Fatalf forwards to sentry
func (f Fields) Fatalf(ctx context.Context, err error, format string, args ...interface{}) {
	fatalf(ctx, err, format, f.kv, args...)
}

// Warnf is Warnf
func (f Fields) Warnf(ctx context.Context, format string, args ...interface{}) {
	warnf(ctx, format, f.kv, args...)
}

// Warn is Warn
func (f Fields) Warn(ctx context.Context, args ...interface{}) {
	Warnf(ctx, "%+v", args...)
}

// Infof is Infof
func (f Fields) Infof(ctx context.Context, format string, args ...interface{}) {
	infof(ctx, format, f.kv, args...)
}

// Info is Info
func (f Fields) Info(ctx context.Context, args ...interface{}) {
	Infof(ctx, "%+v", args...)
}

// Debugf is Debugf
func (f Fields) Debugf(ctx context.Context, format string, args ...interface{}) {
	debugf(ctx, format, f.kv, args...)
}

// Debug is Debug
func (f Fields) Debug(ctx context.Context, args ...interface{}) {
	Debugf(ctx, "%+v", args...)
}

// Errorf forwards to sentry
func (f Fields) Errorf(ctx context.Context, err error, format string, args ...interface{}) {
	errorf(ctx, err, format, f.kv, args...)
}

// Error forwards to sentry
func (f Fields) Error(ctx context.Context, err error, args ...interface{}) {
	Errorf(ctx, err, "%+v", args...)
}
