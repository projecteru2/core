package log

import (
	"context"
	"fmt"
	"runtime/debug"
	"strings"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/getsentry/sentry-go"
	"github.com/projecteru2/core/types"
	"google.golang.org/grpc/peer"
)

// SentryDefer .
func SentryDefer() {
	if sentryDSN == "" {
		return
	}
	defer sentry.Flush(2 * time.Second)
	if r := recover(); r != nil {
		sentry.CaptureMessage(fmt.Sprintf("%+v: %s", r, debug.Stack()))
		panic(r)
	}
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

func reportToSentry(ctx context.Context, level sentry.Level, err error, format string, args ...interface{}) { //nolint
	if sentryDSN == "" {
		return
	}
	defer sentry.Flush(2 * time.Second)
	event, extraDetails := errors.BuildSentryReport(err)
	for k, v := range extraDetails {
		event.Extra[k] = v
	}
	event.Level = level

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
