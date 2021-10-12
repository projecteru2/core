package testsuite

import (
	"os/exec"
	"strings"
)

func bash(command string, env []string) (out string, err error) {
	cmd := exec.Command("/bin/bash", "-c", "set -eo pipefail; "+command)
	cmd.Env = env
	output, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(output)), err
}

func combine(candicates [][]string) (res [][]string) {
	var do func(int, []string)
	do = func(idx int, wip []string) {
		if idx == len(candicates) {
			cp := make([]string, len(wip))
			copy(cp, wip)
			res = append(res, cp)
			return
		}

		wip = append(wip, "")
		for _, s := range candicates[idx] {
			wip[idx] = s
			do(idx+1, wip)
		}
	}

	do(0, []string{})
	return
}
