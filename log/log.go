package log

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/projecteru2/core/types"

	"github.com/getsentry/sentry-go"
	"github.com/rs/zerolog"
	"google.golang.org/grpc/peer"
)

var globalLogger zerolog.Logger

// SetupLog init logger
func SetupLog(l string) error {
	level, err := zerolog.ParseLevel(strings.ToLower(l))
	if err != nil {
		return err
	}
	// TODO can use file
	rslog := zerolog.New(
		zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: time.RFC822,
		}).With().Timestamp().Logger()
	rslog.Level(level)
	globalLogger = rslog

	return nil
}

// Fatalf forwards to sentry
func Fatalf(ctx context.Context, err error, format string, args ...interface{}) {
	sentry.CaptureMessage(fmt.Sprintf(genSentryTracingInfo(ctx)+format, args...))
	globalLogger.Fatal().Err(err).Msgf(format, args...)
}

// Warnf is Warnf
func Warnf(ctx context.Context, format string, args ...interface{}) {
	globalLogger.Warn().Msgf(format, args...)
}

// Warn is Warn
func Warn(ctx context.Context, args ...interface{}) {
	Warnf(ctx, "%v", args...)
}

// Infof is Infof
func Infof(ctx context.Context, format string, args ...interface{}) {
	infoEvent().Msgf(format, args...)
}

// Info is Info
func Info(ctx context.Context, args ...interface{}) {
	Infof(ctx, "%v", args...)
}

// Debugf is Debugf
func Debugf(ctx context.Context, format string, args ...interface{}) {
	globalLogger.Debug().Msgf(format, args...)
}

// Debug is Debug
func Debug(ctx context.Context, args ...interface{}) {
	Debugf(ctx, "%v", args...)
}

// Errorf forwards to sentry
func Errorf(ctx context.Context, err error, format string, args ...interface{}) {
	if err == nil {
		return
	}
	sentry.CaptureMessage(fmt.Sprintf(genSentryTracingInfo(ctx)+format, args...))
	errorEvent(err).Msgf(format, args...)
}

// Error forwards to sentry
func Error(ctx context.Context, err error, args ...interface{}) {
	Errorf(ctx, err, "%v", args...)
}

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

// Infof .
func (f Fields) Infof(ctx context.Context, format string, args ...interface{}) {
	infoEvent().Fields(f.kv).Msgf(format, args...)
}

// Errorf sends sentry message
func (f Fields) Errorf(ctx context.Context, err error, format string, args ...interface{}) {
	if err == nil {
		return
	}
	sentry.CaptureMessage(fmt.Sprintf(genSentryTracingInfo(ctx)+format, args...))
	errorEvent(err).Fields(f.kv).Msgf(format, args...)
}

// for sentry
func genSentryTracingInfo(ctx context.Context) (tracingInfo string) {
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

func infoEvent() *zerolog.Event {
	return globalLogger.Info()
}

func errorEvent(err error) *zerolog.Event {
	if err != nil {
		err = errors.WithStack(err)
	}
	return globalLogger.Error().Err(err)
}
