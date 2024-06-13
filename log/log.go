package log

import (
	"context"
	"io"
	"os"
	"strings"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/getsentry/sentry-go"
	"github.com/projecteru2/core/types"
	"gopkg.in/natefinch/lumberjack.v2"

	"github.com/rs/zerolog"
)

var (
	globalLogger zerolog.Logger
	sentryDSN    string
)

// SetupLog init logger
func SetupLog(ctx context.Context, cfg *types.ServerLogConfig, dsn string) error {
	level, err := zerolog.ParseLevel(strings.ToLower(cfg.Level))
	if err != nil {
		return err
	}

	var writer io.Writer
	switch {
	case cfg.Filename != "":
		// file log always uses json format
		writer = &lumberjack.Logger{
			Filename:   cfg.Filename,
			MaxBackups: cfg.MaxBackups, // files
			MaxSize:    cfg.MaxSize,    // megabytes
			MaxAge:     cfg.MaxAge,     // days
		}
	case !cfg.UseJSON:
		writer = zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: time.RFC822,
		}
	default:
		writer = os.Stdout
	}
	rslog := zerolog.New(writer).With().Timestamp().Logger().Level(level)
	zerolog.ErrorStackMarshaler = func(err error) any {
		return errors.GetSafeDetails(err).SafeDetails
	}
	globalLogger = rslog
	// Sentry
	if dsn != "" {
		sentryDSN = dsn
		WithFunc("log.SetupLog").Infof(ctx, "sentry %v", sentryDSN)
		_ = sentry.Init(sentry.ClientOptions{Dsn: sentryDSN})
	}
	return nil
}

// GetGlobalLogger returns global logger
func GetGlobalLogger() *zerolog.Logger {
	return &globalLogger
}

// Fatalf forwards to sentry
func Fatalf(ctx context.Context, err error, format string, args ...any) {
	fatalf(ctx, err, format, nil, args...)
}

// Warnf is Warnf
func Warnf(ctx context.Context, format string, args ...any) {
	warnf(ctx, format, nil, args...)
}

// Warn is Warn
func Warn(ctx context.Context, args ...any) {
	Warnf(ctx, "%+v", args...)
}

// Infof is Infof
func Infof(ctx context.Context, format string, args ...any) {
	infof(ctx, format, nil, args...)
}

// Info is Info
func Info(ctx context.Context, args ...any) {
	Infof(ctx, "%+v", args...)
}

// Debugf is Debugf
func Debugf(ctx context.Context, format string, args ...any) {
	debugf(ctx, format, nil, args...)
}

// Debug is Debug
func Debug(ctx context.Context, args ...any) {
	Debugf(ctx, "%+v", args...)
}

// Errorf forwards to sentry
func Errorf(ctx context.Context, err error, format string, args ...any) {
	errorf(ctx, err, format, nil, args...)
}

// Error forwards to sentry
func Error(ctx context.Context, err error, args ...any) {
	Errorf(ctx, err, "%+v", args...)
}
