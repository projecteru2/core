package log

import (
	"context"

	"github.com/getsentry/sentry-go"
)

func fatalf(ctx context.Context, err error, format string, fields map[string]interface{}, args ...interface{}) {
	args = argsValidate(args)
	reportToSentry(ctx, sentry.LevelFatal, err, format, args...)
	globalLogger.Fatal().Fields(fields).Err(err).Msgf(format, args...)
}

func warnf(_ context.Context, format string, fields map[string]interface{}, args ...interface{}) {
	args = argsValidate(args)
	globalLogger.Warn().Fields(fields).Msgf(format, args...)
}

func infof(_ context.Context, format string, fields map[string]interface{}, args ...interface{}) {
	args = argsValidate(args)
	globalLogger.Info().Fields(fields).Msgf(format, args...)
}

func debugf(_ context.Context, format string, fields map[string]interface{}, args ...interface{}) {
	args = argsValidate(args)
	globalLogger.Debug().Fields(fields).Msgf(format, args...)
}

func errorf(ctx context.Context, err error, format string, fields map[string]interface{}, args ...interface{}) {
	if err == nil {
		return
	}
	args = argsValidate(args)
	reportToSentry(ctx, sentry.LevelError, err, format, args...)
	globalLogger.Error().Fields(fields).Stack().Err(err).Msgf(format, args...)
}

func argsValidate(args []interface{}) []interface{} {
	if len(args) > 0 {
		return args
	}
	return []interface{}{""}
}
