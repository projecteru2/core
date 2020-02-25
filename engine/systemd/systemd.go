package systemd

import (
	"bytes"
	"context"
	"net"
	"strconv"
	"strings"

	"github.com/projecteru2/core/engine"
	enginetypes "github.com/projecteru2/core/engine/types"
	coretypes "github.com/projecteru2/core/types"
	"golang.org/x/crypto/ssh"
)

const (
	SSHPrefixKey = "systemd://"

	cmdInspectCPUNumber          = "/bin/grep -cz processor /proc/cpuinfo"
	cmdInspectMemoryTotalInBytes = "/usr/bin/awk '/^Mem/ {print $2}' <(/usr/bin/free -bt)"
)

type SystemdSSH struct {
	client *ssh.Client
}

func MakeClient(ctx context.Context, config coretypes.Config, nodename, endpoint, ca, cert, key string) (api engine.API, err error) {
	signer, err := ssh.ParsePrivateKey([]byte(key))
	if err != nil {
		return
	}
	sshConfig := &ssh.ClientConfig{
		User: config.Systemd.Username,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: func(_ string, _ net.Addr, _ ssh.PublicKey) error { return nil },
	}
	sshClient, err := ssh.Dial("tcp", strings.TrimPrefix(endpoint, SSHPrefixKey), sshConfig)
	return &SystemdSSH{sshClient}, err
}

func (s *SystemdSSH) WithSession(f func(*ssh.Session) error) (err error) {
	session, err := s.client.NewSession()
	if err != nil {
		return
	}
	return f(session)
}

func (s *SystemdSSH) Info(ctx context.Context) (info *enginetypes.Info, err error) {
	cpu, err := s.CPUInfo()
	if err != nil {
		return
	}
	memory, err := s.MemoryInfo()
	if err != nil {
		return
	}

	return &enginetypes.Info{
		NCPU:     cpu,
		MemTotal: memory,
	}, nil
}

func (s *SystemdSSH) CPUInfo() (cpu int, err error) {
	stdout := &bytes.Buffer{}
	if err = s.WithSession(func(session *ssh.Session) (err error) {
		session.Stdout = stdout
		return session.Run(cmdInspectCPUNumber)
	}); err != nil {
		return
	}
	return strconv.Atoi(strings.TrimSpace(string(stdout.Bytes())))
}

func (s *SystemdSSH) MemoryInfo() (memoryTotalInBytes int64, err error) {
	stdout := &bytes.Buffer{}
	if err = s.WithSession(func(session *ssh.Session) (err error) {
		session.Stdout = stdout
		return session.Run(cmdInspectMemoryTotalInBytes)
	}); err != nil {
		return
	}
	memory, err := strconv.Atoi(strings.TrimSpace(string(stdout.Bytes())))
	return int64(memory), err
}

func (s *SystemdSSH) ResourceValidate(ctx context.Context, cpu float64, cpumap map[string]int64, memory, storage int64) (err error) {
	return engine.NotImplementedError
}
