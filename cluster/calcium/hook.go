package calcium

import (
	"bytes"
	"context"

	"github.com/projecteru2/core/types"
)

func (c *Calcium) doHook(ctx context.Context, workload *types.Workload, cmds []string, force bool) ([]*bytes.Buffer, error) {
	outputs := []*bytes.Buffer{}
	for _, cmd := range cmds {
		output, err := c.executeInside(ctx, workload.Engine, workload.ID, cmd, workload.User, workload.Env, workload.Privileged)
		if err != nil {
			outputs = append(outputs, bytes.NewBufferString(err.Error()))
			if workload.Hook.Force && !force {
				return outputs, err
			}
			continue
		}
		outputs = append(outputs, bytes.NewBuffer(output))
	}
	return outputs, nil
}
