package log

import (
	"context"

	"github.com/alphadose/haxmap"
	"github.com/getsentry/sentry-go"
	"github.com/rs/zerolog"
)

func fatalf(ctx context.Context, err error, format string, fields *haxmap.Map[string, interface{}], args ...interface{}) {
	args = argsValidate(args)
	reportToSentry(ctx, sentry.LevelFatal, err, format, args...)
	f := globalLogger.Fatal()
	wrap(f, fields).Err(err).Msgf(format, args...)
}

func warnf(_ context.Context, format string, fields *haxmap.Map[string, interface{}], args ...interface{}) {
	args = argsValidate(args)
	f := globalLogger.Warn()
	wrap(f, fields).Msgf(format, args...)
}

func infof(_ context.Context, format string, fields *haxmap.Map[string, interface{}], args ...interface{}) {
	args = argsValidate(args)
	f := globalLogger.Info()
	wrap(f, fields).Msgf(format, args...)
}

func debugf(_ context.Context, format string, fields *haxmap.Map[string, interface{}], args ...interface{}) {
	args = argsValidate(args)
	f := globalLogger.Debug()
	wrap(f, fields).Msgf(format, args...)
}

func errorf(ctx context.Context, err error, format string, fields *haxmap.Map[string, interface{}], args ...interface{}) {
	if err == nil {
		return
	}
	args = argsValidate(args)
	reportToSentry(ctx, sentry.LevelError, err, format, args...)
	f := globalLogger.Error()
	wrap(f, fields).Stack().Err(err).Msgf(format, args...)
}

func argsValidate(args []interface{}) []interface{} {
	if len(args) > 0 {
		return args
	}
	return []interface{}{""}
}

func wrap(f *zerolog.Event, kv *haxmap.Map[string, interface{}]) *zerolog.Event {
	if kv == nil {
		return f
	}
	kv.ForEach(func(k string, v interface{}) bool {
		f = f.Interface(k, v)
		return true
	})
	return f
}
