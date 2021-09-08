package ordeal

import (
	"os/exec"
	"strings"
)

func bash(command string, envs []string) (out string, err error) {
	cmd := exec.Command("/bin/bash", "-c", command)
	cmd.Env = envs
	output, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(output)), err
}
