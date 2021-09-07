package utils

import (
	"context"

	"github.com/projecteru2/core/types"

	"google.golang.org/grpc/peer"
)

// InheritTracingInfo pass through the tracing info: peer, tracing id
func InheritTracingInfo(ctx, newCtx context.Context) context.Context {
	rCtx := newCtx
	if ctx == nil {
		return rCtx
	}

	p, ok := peer.FromContext(ctx)
	if ok {
		rCtx = peer.NewContext(rCtx, p)
	}

	if traceID := ctx.Value(types.TracingID); traceID != nil {
		if tid, ok := traceID.(string); ok {
			rCtx = context.WithValue(rCtx, types.TracingID, tid)
		}
	}

	return rCtx
}
