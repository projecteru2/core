package log

import (
	"fmt"
	"os"

	"github.com/getsentry/sentry-go"
	log "github.com/sirupsen/logrus"
)

// SetupLog init logger
func SetupLog(l string) error {
	level, err := log.ParseLevel(l)
	if err != nil {
		return err
	}
	log.SetLevel(level)

	formatter := &log.TextFormatter{
		TimestampFormat: "2006-01-02 15:04:05",
		FullTimestamp:   true,
	}
	log.SetFormatter(formatter)
	log.SetOutput(os.Stdout)
	return nil
}

// Error forwards to sentry
func Error(args ...interface{}) {
	sentry.CaptureMessage(fmt.Sprint(args...))
	log.Error(args...)
}

// Errorf forwards to sentry
func Errorf(format string, args ...interface{}) {
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
func Warnf(format string, args ...interface{}) {
	log.Warnf(format, args...)
}

// Info is Info
func Info(args ...interface{}) {
	log.Info(args...)
}

// Infof is Infof
func Infof(format string, args ...interface{}) {
	log.Infof(format, args...)
}

// Debug is Debug
func Debug(args ...interface{}) {
	log.Debug(args...)
}

// Debugf is Debugf
func Debugf(format string, args ...interface{}) {
	log.Debugf(format, args...)
}
