package log

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/getsentry/sentry-go"

	"github.com/rs/zerolog"
)

var (
	globalLogger zerolog.Logger
	sentryDSN    string
)

// SetupLog init logger
func SetupLog(ctx context.Context, l, dsn string) error {
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
	// Sentry
	if dsn != "" {
		sentryDSN = dsn
		Infof(ctx, "[log] sentry %+v", sentryDSN)
		_ = sentry.Init(sentry.ClientOptions{Dsn: sentryDSN})
	}
	return nil
}

// Fatalf forwards to sentry
func Fatalf(ctx context.Context, err error, format string, args ...interface{}) {
	fatalf(ctx, err, format, nil, args...)
}

// Warnf is Warnf
func Warnf(ctx context.Context, format string, args ...interface{}) {
	warnf(ctx, format, nil, args...)
}

// Warn is Warn
func Warn(ctx context.Context, args ...interface{}) {
	Warnf(ctx, "%+v", args...)
}

// Infof is Infof
func Infof(ctx context.Context, format string, args ...interface{}) {
	infof(ctx, format, nil, args...)
}

// Info is Info
func Info(ctx context.Context, args ...interface{}) {
	Infof(ctx, "%+v", args...)
}

// Debugf is Debugf
func Debugf(ctx context.Context, format string, args ...interface{}) {
	debugf(ctx, format, nil, args...)
}

// Debug is Debug
func Debug(ctx context.Context, args ...interface{}) {
	Debugf(ctx, "%+v", args...)
}

// Errorf forwards to sentry
func Errorf(ctx context.Context, err error, format string, args ...interface{}) {
	errorf(ctx, err, format, nil, args...)
}

// Error forwards to sentry
func Error(ctx context.Context, err error, args ...interface{}) {
	Errorf(ctx, err, "%+v", args...)
}
