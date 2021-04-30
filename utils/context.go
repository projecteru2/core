package utils

import (
	"context"

	"google.golang.org/grpc/peer"
)

func InheritTracingInfo(ctx, newCtx context.Context) context.Context {
	rCtx := newCtx

	p, ok := peer.FromContext(ctx)
	if ok {
		rCtx = peer.NewContext(rCtx, p)
	}

	if traceID := ctx.Value("traceID"); traceID != nil {
		if tid, ok := traceID.(string); ok {
			rCtx = context.WithValue(rCtx, "traceID", tid)
		}
	}

	return rCtx
}
