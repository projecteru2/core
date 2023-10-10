package strategy

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func deployedNodes() []Info {
	return []Info{
		{
			Nodename: "n1",
			Capacity: 10,
			Count:    2,
		},
		{
			Nodename: "n2",
			Capacity: 10,
			Count:    3,
		},
		{
			Nodename: "n3",
			Capacity: 10,
			Count:    5,
		},
		{
			Nodename: "n4",
			Capacity: 10,
			Count:    7,
		},
	}
}

func TestDeploy(t *testing.T) {
	ctx := context.Background()

	// invalid strategy
	_, err := Deploy(ctx, "invalid", -1, 3, nil, 2)
	assert.Error(t, err)

	// count < 0
	_, err = Deploy(ctx, "AUTO", -1, 3, nil, 2)
	assert.Error(t, err)

	Plans["test"] = func(_ context.Context, _ []Info, _, _, _ int) (map[string]int, error) {
		return nil, nil
	}
	_, err = Deploy(ctx, "test", 1, 3, nil, 2)
	assert.NoError(t, err)
}
