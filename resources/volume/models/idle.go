package models

import (
	"context"
)

const priority = 1

// GetMostIdleNode .
func (v *Volume) GetMostIdleNode(ctx context.Context, nodes []string) (string, int, error) {
	return "", priority, nil
}
