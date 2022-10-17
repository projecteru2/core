package calcium

import (
	"bytes"
	"context"

	"github.com/projecteru2/core/engine"
)

func (c *Calcium) doHook(
	ctx context.Context,
	ID, user string,
	cmds, env []string,
	cmdForce, privileged, force bool,
	engine engine.API,
) ([]*bytes.Buffer, error) {
	outputs := []*bytes.Buffer{}
	for _, cmd := range cmds {
		output, err := c.execuateInside(ctx, engine, ID, cmd, user, env, privileged)
		if err != nil {
			// 执行 hook 的过程中,如果 cmdForce 为真并且不忽略 hook 就输出错误
			outputs = append(outputs, bytes.NewBufferString(err.Error()))
			if cmdForce && !force {
				return outputs, err
			}
			continue
		}
		outputs = append(outputs, bytes.NewBuffer(output))
	}
	return outputs, nil
}
