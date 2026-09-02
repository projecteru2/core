package binary

import (
	"bytes"
	"context"
	"encoding/json"
	"os/exec"
	"slices"
	"strings"

	"github.com/cockroachdb/errors"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/resource/plugins"
)

func (p Plugin) call(ctx context.Context, cmd string, req, resp any) error {
	if !slices.Contains(p.verbs, cmd) {
		return plugins.ErrVerbNotSupported
	}
	ctx, cancel := context.WithTimeout(ctx, p.config.ResourcePlugin.CallTimeout)
	defer cancel()
	logger := log.WithFunc("resource.binary.call").WithField("plugin", p.name).WithField("cmd", cmd)

	command := exec.CommandContext(ctx, p.path, cmd) //nolint:gosec // the plugin path comes from the operator's own config
	command.Dir = p.config.ResourcePlugin.Dir

	b, err := json.Marshal(req)
	if err != nil {
		return err
	}
	if cmd != GetMetricsCommand {
		logger.WithField("in", string(b)).Debug(ctx, "call params")
	}
	command.Stdin = bytes.NewBuffer(b)
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		return errors.Wrapf(err, "plugin %s %s: %s", p.name, cmd, strings.TrimSpace(stderr.String()))
	}
	if stderr.Len() > 0 {
		logger.Debug(ctx, stderr.String())
	}
	if stdout.Len() == 0 {
		return errors.Newf("plugin %s wrote no response for %s", p.name, cmd)
	}
	return json.Unmarshal(stdout.Bytes(), resp)
}
