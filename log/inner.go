package log

import (
	"context"
	"strings"

	"github.com/alphadose/haxmap"
	"github.com/getsentry/sentry-go"
	"github.com/rs/zerolog"
)

func fatalf(ctx context.Context, err error, format string, fields *haxmap.Map[string, any], args ...any) {
	reportToSentry(ctx, sentry.LevelFatal, err, format, args...)
	f := globalLogger.Fatal()
	wrap(f, fields).Err(err).Msgf(format, args...)
}

func warnf(_ context.Context, format string, fields *haxmap.Map[string, any], args ...any) {
	f := globalLogger.Warn()
	wrap(f, fields).Msgf(format, args...)
}

func infof(_ context.Context, format string, fields *haxmap.Map[string, any], args ...any) {
	f := globalLogger.Info()
	wrap(f, fields).Msgf(format, args...)
}

func debugf(_ context.Context, format string, fields *haxmap.Map[string, any], args ...any) {
	f := globalLogger.Debug()
	wrap(f, fields).Msgf(format, args...)
}

func errorf(ctx context.Context, err error, format string, fields *haxmap.Map[string, any], args ...any) {
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

func wrap(f *zerolog.Event, kv *haxmap.Map[string, any]) *zerolog.Event {
	if kv == nil {
		return f
	}
	kv.ForEach(func(k string, v any) bool {
		f = f.Interface(k, v)
		return true
	})
	return f
}
