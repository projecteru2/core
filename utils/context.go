package utils

import (
	"context"
)

// NewInheritCtx returns ctx with its values but without its cancellation.
func NewInheritCtx(ctx context.Context) context.Context {
	return context.WithoutCancel(ctx)
}
