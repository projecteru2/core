package log

import (
	"context"
	"strings"

	"github.com/getsentry/sentry-go"
	"github.com/rs/zerolog"
)

func fatalf(ctx context.Context, err error, format string, fields []field, args ...any) {
	reportToSentry(ctx, sentry.LevelFatal, err, format, args...)
	f := globalLogger.Fatal()
	wrap(f, fields).Err(err).Msgf(format, args...)
}

func warnf(_ context.Context, format string, fields []field, args ...any) {
	f := globalLogger.Warn()
	wrap(f, fields).Msgf(format, args...)
}

func infof(_ context.Context, format string, fields []field, args ...any) {
	f := globalLogger.Info()
	wrap(f, fields).Msgf(format, args...)
}

func debugf(_ context.Context, format string, fields []field, args ...any) {
	f := globalLogger.Debug()
	wrap(f, fields).Msgf(format, args...)
}

func errorf(ctx context.Context, err error, format string, fields []field, args ...any) {
	if err == nil {
		return
	}
	reportToSentry(ctx, sentry.LevelError, err, format, args...)
	f := globalLogger.Error()
	wrap(f, fields).Stack().Err(err).Msgf(format, args...)
}

func formatArgs(args []any) string {
	if len(args) == 0 {
		return ""
	}
	return strings.TrimSuffix(strings.Repeat("%+v ", len(args)), " ")
}

func wrap(f *zerolog.Event, kv []field) *zerolog.Event {
	for _, e := range kv {
		f = f.Interface(e.key, e.value)
	}
	return f
}
