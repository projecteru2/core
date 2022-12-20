package binary

import (
	"bytes"
	"context"
	"encoding/json"
	"os/exec"
	"strings"

	"github.com/projecteru2/core/log"
)

// calls the plugin and gets json response
func (p Plugin) call(ctx context.Context, cmd string, req interface{}, resp interface{}) error {
	ctx, cancel := context.WithTimeout(ctx, p.config.ResourcePlugin.CallTimeout)
	defer cancel()
	logger := log.WithFunc("resource.binary.call")

	args, err := getArgs(req)
	if err != nil {
		logger.Error(ctx, err)
	}
	args = append([]string{cmd}, args...)
	command := exec.CommandContext(ctx, p.path, args...) //nolint: gosec
	command.Dir = p.config.ResourcePlugin.Dir
	logger.Infof(ctx, "command: %s %s", p.path, strings.Join(args, " "))

	stdout, stderr, err := p.execCommand(command)
	if err != nil {
		logger.Errorf(ctx, err, "failed to run plugin %s, command %+v", p.path, args)
		return err
	}

	stdoutBytes := stdout.Bytes()
	if len(stdoutBytes) == 0 {
		return nil
	}
	defer logger.Infof(ctx, "stdout from plugin %s: %s", p.path, string(stdoutBytes))
	defer logger.Infof(ctx, "stderr from plugin %s: %s", p.path, stderr.String())

	if err := json.Unmarshal(stdoutBytes, resp); err != nil {
		logger.Errorf(ctx, err, "failed to unmarshal stdout of plugin %s, command %+v, output %s", p.path, args, string(stdoutBytes))
		return err
	}

	return nil
}

func (p Plugin) execCommand(cmd *exec.Cmd) (output, log bytes.Buffer, err error) {
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	return stdout, stderr, cmd.Run()
}
