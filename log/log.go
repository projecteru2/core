package log

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/projecteru2/core/types"

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
	zerolog.ErrorStackMarshaler = func(err error) interface{} {
		return errors.GetSafeDetails(err).SafeDetails
	}
	globalLogger = rslog

	return nil
}

// Fatalf forwards to sentry
func Fatalf(ctx context.Context, err error, format string, args ...interface{}) {
	args = argsValidate(args)
	reportToSentry(ctx, err, format, args)
	globalLogger.Fatal().Err(err).Msgf(format, args...)
}

// Warnf is Warnf
func Warnf(ctx context.Context, format string, args ...interface{}) {
	args = argsValidate(args)
	globalLogger.Warn().Msgf(format, args...)
}

// Warn is Warn
func Warn(ctx context.Context, args ...interface{}) {
	Warnf(ctx, "%+v", args...)
}

// Infof is Infof
func Infof(ctx context.Context, format string, args ...interface{}) {
	args = argsValidate(args)
	infoEvent().Msgf(format, args...)
}

// Info is Info
func Info(ctx context.Context, args ...interface{}) {
	Infof(ctx, "%+v", args...)
}

// Debugf is Debugf
func Debugf(ctx context.Context, format string, args ...interface{}) {
	args = argsValidate(args)
	globalLogger.Debug().Msgf(format, args...)
}

// Debug is Debug
func Debug(ctx context.Context, args ...interface{}) {
	Debugf(ctx, "%+v", args...)
}

// Errorf forwards to sentry
func Errorf(ctx context.Context, err error, format string, args ...interface{}) {
	args = argsValidate(args)
	if err == nil {
		return
	}
	reportToSentry(ctx, err, format, args)
	errorEvent(err).Msgf(format, args...)
}

// Error forwards to sentry
func Error(ctx context.Context, err error, args ...interface{}) {
	Errorf(ctx, err, "%+v", args...)
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
	args = argsValidate(args)
	infoEvent().Fields(f.kv).Msgf(format, args...)
}

// Errorf sends sentry message
func (f Fields) Errorf(ctx context.Context, err error, format string, args ...interface{}) {
	args = argsValidate(args)
	if err == nil {
		return
	}
	reportToSentry(ctx, err, format, args)
	errorEvent(err).Fields(f.kv).Msgf(format, args...)
}

// Error forwards to sentry
func (f Fields) Error(ctx context.Context, err error, args ...interface{}) {
	f.Errorf(ctx, err, "%+v", args...)
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
	return globalLogger.Error().Stack().Err(err)
}

func argsValidate(args []interface{}) []interface{} {
	if len(args) > 0 {
		return args
	}
	return []interface{}{""}
}

func reportToSentry(ctx context.Context, err error, format string, args ...interface{}) { //nolint
	err = errors.Wrap(err, fmt.Sprintf(genSentryTracingInfo(ctx)+format, args...))
	if eventID := errors.ReportError(err); eventID != "" {
		Infof(ctx, "Report to Sentry ID: %s", eventID)
	}
}
