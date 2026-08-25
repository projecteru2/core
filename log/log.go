package log

import (
	"context"
	"io"
	"os"
	"strings"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/getsentry/sentry-go"
	"github.com/rs/zerolog"
	"gopkg.in/natefinch/lumberjack.v2"

	"github.com/projecteru2/core/types"
)

var (
	globalLogger zerolog.Logger
	sentryDSN    string
)

// SetupLog configures the global logger and the optional Sentry client.
func SetupLog(ctx context.Context, cfg *types.ServerLogConfig, dsn string) error {
	level, err := zerolog.ParseLevel(strings.ToLower(cfg.Level))
	if err != nil {
		return err
	}

	rslog := zerolog.New(logWriter(cfg)).With().Timestamp().Logger().Level(level)
	zerolog.ErrorStackMarshaler = func(err error) any {
		return errors.GetSafeDetails(err).SafeDetails
	}
	globalLogger = rslog
	if dsn != "" {
		if err := sentry.Init(sentry.ClientOptions{Dsn: dsn}); err != nil {
			return err
		}
		sentryDSN = dsn
		WithFunc("log.SetupLog").Info(ctx, "sentry enabled")
	}
	return nil
}

func GetGlobalLogger() *zerolog.Logger {
	return &globalLogger
}

// Fatalf logs at fatal level and reports to Sentry.
func Fatalf(ctx context.Context, err error, format string, args ...any) {
	fatalf(ctx, err, format, nil, args...)
}

func Warnf(ctx context.Context, format string, args ...any) {
	warnf(ctx, format, nil, args...)
}

func Warn(ctx context.Context, args ...any) {
	Warnf(ctx, formatArgs(args), args...)
}

func Infof(ctx context.Context, format string, args ...any) {
	infof(ctx, format, nil, args...)
}

func Info(ctx context.Context, args ...any) {
	Infof(ctx, formatArgs(args), args...)
}

func Debugf(ctx context.Context, format string, args ...any) {
	debugf(ctx, format, nil, args...)
}

func Debug(ctx context.Context, args ...any) {
	Debugf(ctx, formatArgs(args), args...)
}

// Errorf logs at error level and reports to Sentry.
func Errorf(ctx context.Context, err error, format string, args ...any) {
	errorf(ctx, err, format, nil, args...)
}

// Error logs at error level and reports to Sentry.
func Error(ctx context.Context, err error, args ...any) {
	Errorf(ctx, err, formatArgs(args), args...)
}

func logWriter(cfg *types.ServerLogConfig) io.Writer {
	switch {
	case cfg.Filename != "":
		// file log always uses json format
		return &lumberjack.Logger{
			Filename:   cfg.Filename,
			MaxBackups: cfg.MaxBackups,
			MaxSize:    cfg.MaxSize, // megabytes
			MaxAge:     cfg.MaxAge,  // days
		}
	case !cfg.UseJSON:
		return zerolog.ConsoleWriter{
			Out:        os.Stderr,
			TimeFormat: time.RFC822,
		}
	default:
		return os.Stderr
	}
}
