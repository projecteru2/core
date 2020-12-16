package systemd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"github.com/projecteru2/core/log"

	"github.com/docker/go-units"
	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

const (
	unitTemplate = `
[Unit]
%s

[Service]
%s
	`
)

type unitBuilder struct {
	ID            string
	opts          *enginetypes.VirtualizationCreateOptions
	unitBuffer    []string
	serviceBuffer []string
	err           error
}

type unitDesciption struct {
	ID     string
	Name   string
	Labels map[string]string
}

func (s *SSHClient) newUnitBuilder(ID string, opts *enginetypes.VirtualizationCreateOptions) *unitBuilder {
	return &unitBuilder{
		ID:   ID,
		opts: opts,
	}
}

func (b *unitBuilder) cgroupPath() string {
	return b.ID
}

func (b *unitBuilder) buildUnit() *unitBuilder {
	if b.err != nil {
		return b
	}

	description, err := json.Marshal(unitDesciption{Name: b.opts.Name, Labels: b.opts.Labels})
	if err != nil {
		b.err = err
		return b
	}

	b.unitBuffer = append(b.unitBuffer, []string{
		fmt.Sprintf("Description=%s", string(description)),
		"After=network-online.target firewalld.service",
		"Wants=network-online.target",
	}...)
	return b
}

func (b *unitBuilder) buildPreExec(cpuAmount int) *unitBuilder {
	if b.err != nil {
		return b
	}

	b.serviceBuffer = append(b.serviceBuffer,
		fmt.Sprintf("ExecStartPre=/usr/bin/cgcreate -g memory,cpuset:%s", b.cgroupPath()),
	)

	return b.buildNetworkLimit().buildCPULimit(cpuAmount).buildMemoryLimit()
}

func (b *unitBuilder) buildNetworkLimit() *unitBuilder {
	if b.err != nil {
		return b
	}

	network := ""
	for net := range b.opts.Networks {
		if net != "" {
			network = net
			break
		}
	}

	if network != "" && network != "host" {
		b.err = errors.Wrapf(types.ErrEngineNotImplemented, "systemd engine doesn't support network %s", network)
		return b
	}
	return b
}

func (b *unitBuilder) buildCPULimit(cpuAmount int) *unitBuilder {
	if b.err != nil {
		return b
	}

	cpusetCPUs := fmt.Sprintf("0-%d", cpuAmount-1)
	if len(b.opts.CPU) > 0 {
		allowedCPUs := []string{}
		for CPU := range b.opts.CPU {
			allowedCPUs = append(allowedCPUs, CPU)
		}
		cpusetCPUs = strings.Join(allowedCPUs, ",")
	}

	if b.opts.Quota > 0 {
		b.serviceBuffer = append(b.serviceBuffer,
			fmt.Sprintf("CPUQuota=%.2f%%", b.opts.Quota*100),
		)
	}

	numaNode := b.opts.NUMANode
	if numaNode == "" {
		numaNode = "0"
	}
	b.serviceBuffer = append(b.serviceBuffer,
		fmt.Sprintf("ExecStartPre=/usr/bin/cgset -r cpuset.cpus=%s %s", cpusetCPUs, b.cgroupPath()),
		fmt.Sprintf("ExecStartPre=/usr/bin/cgset -r cpuset.mems=%s %s", numaNode, b.cgroupPath()),
	)

	return b
}
func (b *unitBuilder) buildMemoryLimit() *unitBuilder {
	if b.err != nil {
		return b
	}

	if b.opts.Memory == 0 {
		return b
	}

	//	if b.opts.SoftLimit {
	//		b.serviceBuffer = append(b.serviceBuffer,
	//			fmt.Sprintf("ExecStartPre=/usr/bin/cgset -r memory.soft_limit_in_bytes=%d %s", b.opts.Memory, b.cgroupPath()),
	//		)
	//
	//	} else {
	b.serviceBuffer = append(b.serviceBuffer,
		fmt.Sprintf("ExecStartPre=/usr/bin/cgset -r memory.limit_in_bytes=%d %s", b.opts.Memory, b.cgroupPath()),
		fmt.Sprintf("ExecStartPre=/usr/bin/cgset -r memory.soft_limit_in_bytes=%d %s", utils.Max(int(b.opts.Memory/2), units.MiB*4), b.cgroupPath()),
	)
	//	}
	return b
}

func (b *unitBuilder) buildExec() *unitBuilder {
	if b.err != nil {
		return b
	}

	user := b.opts.User
	if user == "" {
		user = "root"
	}

	env := []string{}
	for _, e := range b.opts.Env {
		env = append(env, fmt.Sprintf(`"%s"`, e))
	}

	stdioType, err := b.convertToSystemdStdio(b.opts.LogType)
	if err != nil {
		b.err = err
		return b
	}

	restartPolicy, err := b.convertToSystemdRestartPolicy(b.opts.RestartPolicy)
	if err != nil {
		b.err = err
		return b
	}

	cmds := []string{}
	for _, cmd := range b.opts.Cmd {
		if strings.Contains(cmd, " ") {
			cmd = fmt.Sprintf("'%s'", cmd)
		}
		cmds = append(cmds, cmd)
	}

	b.serviceBuffer = append(b.serviceBuffer, []string{
		fmt.Sprintf("ExecStart=/usr/bin/cgexec -g memory,cpuset:%s %s", b.cgroupPath(), strings.Join(cmds, " ")),
		fmt.Sprintf("User=%s", user),
		fmt.Sprintf("Environment=%s", strings.Join(env, " ")),
		fmt.Sprintf("StandardOutput=%s", stdioType),
		fmt.Sprintf("StandardError=%s", stdioType),
		fmt.Sprintf("Restart=%s", restartPolicy),
	}...)
	return b
}

func (b *unitBuilder) buildPostExec() *unitBuilder {
	if b.err != nil {
		return b
	}

	b.serviceBuffer = append(b.serviceBuffer,
		fmt.Sprintf("ExecStopPost=/usr/bin/cgdelete -g cpuset,memory:%s", b.cgroupPath()),
	)
	return b
}

func (b *unitBuilder) buffer() (*bytes.Buffer, error) {
	unit := fmt.Sprintf(unitTemplate,
		strings.Join(b.unitBuffer, "\n"),
		strings.Join(b.serviceBuffer, "\n"),
	)
	log.Debugf("%s", unit)
	return bytes.NewBufferString(unit), b.err
}

func (b *unitBuilder) convertToSystemdRestartPolicy(restart string) (policy string, err error) {
	switch {
	case restart == "no":
		policy = "no"
	case restart == "always" || restart == "":
		policy = "always"
	case strings.HasPrefix(restart, "on-failure"):
		policy = "on-failure"
	default:
		err = fmt.Errorf("restart policy not supported: %s", restart)
	}
	return
}

func (b *unitBuilder) convertToSystemdStdio(logType string) (stdioType string, err error) {
	switch logType {
	case "journald", "":
		stdioType = "journal"
	case "none":
		stdioType = "null"
	default:
		err = fmt.Errorf("log type not supported: %s", logType)
	}
	return
}
