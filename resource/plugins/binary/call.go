package binary

import (
	"bytes"
	"context"
	"encoding/json"
	"os/exec"

	"github.com/projecteru2/core/log"
)

func (p Plugin) call(ctx context.Context, cmd string, req, resp any) error {
	ctx, cancel := context.WithTimeout(ctx, p.config.ResourcePlugin.CallTimeout)
	defer cancel()
	logger := log.WithFunc("resource.binary.call")

	command := exec.CommandContext(ctx, p.path, cmd) //nolint:gosec // the plugin path comes from the operator's own config
	command.Dir = p.config.ResourcePlugin.Dir

	b, err := json.Marshal(req)
	if err != nil {
		return err
	}
	if cmd != GetMetricsCommand {
		logger.WithField("in", string(b)).WithField("cmd", command.String()).Debug(ctx, "call params")
	}
	command.Stdin = bytes.NewBuffer(b)
	out, err := command.CombinedOutput()
	if err != nil {
		logger.Error(ctx, err, string(out))
		return err
	}

	if len(out) == 0 {
		return nil
	}
	return json.Unmarshal(out, resp)
}
