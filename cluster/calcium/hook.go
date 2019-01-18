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
	cmdForce, force, privileged bool,
	engine engine.API,
) ([]*bytes.Buffer, error) {
	outputs := []*bytes.Buffer{}
	for _, cmd := range cmds {
		output, err := execuateInside(ctx, engine, ID, cmd, user, env, privileged)
		if err != nil {
			// force 指是否强制删除，所以这里即便 hook 是强制执行，但是失败了，也不应该影响删除过程
			// start 的过程中，force 永远都是 false，保证由 hook 的 force 来定义逻辑
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
