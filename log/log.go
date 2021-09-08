package log

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/projecteru2/core/types"

	"github.com/getsentry/sentry-go"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc/peer"
)

// SetupLog init logger
func SetupLog(l string) error {
	level, err := log.ParseLevel(l)
	if err != nil {
		return err
	}
	log.SetLevel(level)

	formatter := &log.TextFormatter{
		ForceColors:     true,
		TimestampFormat: "2006-01-02 15:04:05",
		FullTimestamp:   true,
	}
	log.SetFormatter(formatter)
	log.SetOutput(os.Stdout)
	return nil
}

// Fields is a wrapper for logrus.Entry
// we need to insert some sentry captures here
type Fields struct {
	e *log.Entry
}

// WithField .
func (f Fields) WithField(key string, value interface{}) Fields {
	e := f.e.WithField(key, value)
	return Fields{e: e}
}

// Errorf sends sentry message
func (f Fields) Errorf(ctx context.Context, format string, args ...interface{}) {
	format = getTracingInfo(ctx) + format
	sentry.CaptureMessage(fmt.Sprintf(format, args...))
	f.e.Errorf(format, args...)
}

// Err is a decorator returning the argument
func (f Fields) Err(ctx context.Context, err error) error {
	format := getTracingInfo(ctx) + "%+v"
	if err != nil {
		sentry.CaptureMessage(fmt.Sprintf(format, err))
		f.e.Errorf(format, err)
	}
	return err
}

// WithField add kv into log entry
func WithField(key string, value interface{}) Fields {
	return Fields{
		e: log.WithField(key, value),
	}
}

// Error forwards to sentry
func Error(args ...interface{}) {
	sentry.CaptureMessage(fmt.Sprint(args...))
	log.Error(args...)
}

// Errorf forwards to sentry
func Errorf(ctx context.Context, format string, args ...interface{}) {
	format = getTracingInfo(ctx) + format
	sentry.CaptureMessage(fmt.Sprintf(format, args...))
	log.Errorf(format, args...)
}

// Fatalf forwards to sentry
func Fatalf(format string, args ...interface{}) {
	sentry.CaptureMessage(fmt.Sprintf(format, args...))
	log.Fatalf(format, args...)
}

// Warn is Warn
func Warn(args ...interface{}) {
	log.Warn(args...)
}

// Warnf is Warnf
func Warnf(ctx context.Context, format string, args ...interface{}) {
	log.Warnf(getTracingInfo(ctx)+format, args...)
}

// Info is Info
func Info(args ...interface{}) {
	log.Info(args...)
}

// Infof is Infof
func Infof(ctx context.Context, format string, args ...interface{}) {
	log.Infof(getTracingInfo(ctx)+format, args...)
}

// Debug is Debug
func Debug(ctx context.Context, args ...interface{}) {
	a := []interface{}{getTracingInfo(ctx)}
	a = append(a, args...)
	log.Debug(a)
}

// Debugf is Debugf
func Debugf(ctx context.Context, format string, args ...interface{}) {
	log.Debugf(getTracingInfo(ctx)+format, args...)
}

func getTracingInfo(ctx context.Context) (tracingInfo string) {
	if ctx == nil {
		return ""
	}

	tracing := []string{}
	if p, ok := peer.FromContext(ctx); ok {
		tracing = append(tracing, p.Addr.String())
	}

	if traceID := ctx.Value(types.TracingID); traceID != nil {
		if tid, ok := traceID.(string); ok {
			tracing = append(tracing, tid)
		}
	}
	tracingInfo = strings.Join(tracing, "-")
	if tracingInfo != "" {
		tracingInfo = fmt.Sprintf("[%s] ", tracingInfo)
	}
	return
}
