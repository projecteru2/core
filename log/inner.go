package log

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/getsentry/sentry-go"
	"github.com/projecteru2/core/types"
	"google.golang.org/grpc/peer"
)

func fatalf(ctx context.Context, err error, format string, fields map[string]interface{}, args ...interface{}) {
	args = argsValidate(args)
	reportToSentry(ctx, err, format, args...)
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
	reportToSentry(ctx, err, format, args...)
	globalLogger.Error().Fields(fields).Stack().Err(err).Msgf(format, args...)
}

func argsValidate(args []interface{}) []interface{} {
	if len(args) > 0 {
		return args
	}
	return []interface{}{""}
}

func genGRPCTracingInfo(ctx context.Context) (tracingInfo string) {
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

func reportToSentry(ctx context.Context, err error, format string, args ...interface{}) { //nolint
	defer sentry.Flush(2 * time.Second)
	event, extraDetails := errors.BuildSentryReport(err)
	for extraKey, extraValue := range extraDetails {
		event.Extra[extraKey] = extraValue
	}
	event.Tags["report_type"] = "error"

	if msg := fmt.Sprintf(format, args...); msg != "" {
		event.Tags["message"] = msg
	}

	if tracingInfo := genGRPCTracingInfo(ctx); tracingInfo != "" {
		event.Tags["tracing"] = tracingInfo
	}

	if res := string(*sentry.CaptureEvent(event)); res != "" {
		Infof(ctx, "Report to Sentry ID: %s", res)
	}
}
