package systemd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"

	enginetypes "github.com/projecteru2/core/engine/types"
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
	opts          *enginetypes.VirtualizationCreateOptions
	unitBuffer    []string
	serviceBuffer []string
	err           error
}

type unitDesciption struct {
	Name   string
	Labels map[string]string
}

func (s *SystemdSSH) newUnitBuilder(opts *enginetypes.VirtualizationCreateOptions) *unitBuilder {
	return &unitBuilder{
		opts: opts,
	}
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

func (b *unitBuilder) buildResource(CPUAmount int) *unitBuilder {
	if b.err != nil {
		return b
	}

	allowedCPUs := []string{}
	for CPU, _ := range b.opts.CPU {
		allowedCPUs = append(allowedCPUs, CPU)
	}

	CPUQuotaPercentage := b.opts.Quota / float64(CPUAmount) * 100

	b.serviceBuffer = append(b.serviceBuffer, []string{
		fmt.Sprintf("ExecStartPre=/usr/bin/cgcreate -g memory,cpuset:%s", b.opts.Name),
		fmt.Sprintf("ExecStartPre=/usr/bin/cgset -r cpuset.cpus=%s %s", strings.Join(allowedCPUs, ","), b.opts.Name),
		fmt.Sprintf("ExecStartPre=/usr/bin/cgset -r cpuset.mems=0 %s", b.opts.Name),
		fmt.Sprintf("ExecStartPre=/usr/bin/cgset -r memory.limit_in_bytes=%d %s", b.opts.Memory, b.opts.Name),
		fmt.Sprintf("CPUQuota=%.2f%%", CPUQuotaPercentage),
	}...)
	return b
}

func (b *unitBuilder) buildExec() *unitBuilder {
	if b.err != nil {
		return b
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

	b.serviceBuffer = append(b.serviceBuffer, []string{
		fmt.Sprintf("ExecStart=/usr/bin/cgexec -g memory,cpuset:%s %s", b.opts.Name, strings.Join(b.opts.Cmd, " ")),
		fmt.Sprintf("StandardOutput=%s", stdioType),
		fmt.Sprintf("StandardError=%s", stdioType),
		fmt.Sprintf("Restart=%s", restartPolicy),
	}...)
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
