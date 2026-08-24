package utils

import (
	"context"

	"google.golang.org/grpc/peer"

	"github.com/projecteru2/core/types"
)

// NewInheritCtx returns ctx with its values but without its cancellation.
func NewInheritCtx(ctx context.Context) context.Context {
	return context.WithoutCancel(ctx)
}

// InheritTracingInfo copies the peer and tracing ID from ctx onto newCtx.
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
