package strategy

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDeploy(t *testing.T) {
	ctx := t.Context()

	_, err := Deploy(ctx, "invalid", -1, 3, nil, 2)
	assert.Error(t, err)

	_, err = Deploy(ctx, "AUTO", -1, 3, nil, 2)
	assert.Error(t, err)

	Plans["test"] = func(_ context.Context, _ []Info, _, _, _ int) (map[string]int, error) {
		return nil, nil
	}
	t.Cleanup(func() { delete(Plans, "test") })
	_, err = Deploy(ctx, "test", 1, 3, nil, 2)
	assert.NoError(t, err)
}

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
