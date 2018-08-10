package calcium

import (
	"context"

	"github.com/projecteru2/core/types"
)

func (c *Calcium) doContainerBeforeStopHook(
	ctx context.Context,
	container *types.Container,
	user string,
	env []string,
	privileged bool,
	force bool,
) ([]byte, error) {
	outputs := []byte{}
	for _, cmd := range container.Hook.BeforeStop {
		output, err := execuateInside(ctx, container.Engine, container.ID, cmd, user, env, privileged)
		if err != nil {
			// force 指是否强制删除，所以这里即便 hook 是强制执行，但是失败了，也不应该影响删除过程
			if container.Hook.Force && !force {
				return outputs, err
			}
			outputs = append(outputs, []byte(err.Error())...)
			continue
		}
		outputs = append(outputs, output...)
	}
	return outputs, nil
}

func (c *Calcium) doContainerAfterStartHook(
	ctx context.Context,
	container *types.Container,
	user string,
	env []string,
	privileged bool,
) ([]byte, error) {
	outputs := []byte{}
	for _, cmd := range container.Hook.AfterStart {
		output, err := execuateInside(ctx, container.Engine, container.ID, cmd, user, env, privileged)
		if err != nil {
			if container.Hook.Force {
				return outputs, err
			}
			outputs = append(outputs, []byte(err.Error())...)
			continue
		}
		outputs = append(outputs, output...)
	}
	return outputs, nil
}
