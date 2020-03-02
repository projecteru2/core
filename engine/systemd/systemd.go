package systemd

import (
	"bytes"
	"context"
	"io"
	"net"
	"strconv"
	"strings"

	"github.com/pkg/errors"
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
	hostIP string
	client *ssh.Client
}

func NewSystemdSSH(endpoint string, config *ssh.ClientConfig) (*SystemdSSH, error) {
	parts := strings.Split(endpoint, ":")
	client, err := ssh.Dial("tcp", endpoint, config)
	return &SystemdSSH{
		hostIP: parts[0],
		client: client,
	}, err
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
	return NewSystemdSSH(
		strings.TrimPrefix(endpoint, SSHPrefixKey),
		sshConfig,
	)
}

func (s *SystemdSSH) WithSession(f func(*ssh.Session) error) (err error) {
	session, err := s.client.NewSession()
	defer session.Close()
	if err != nil {
		return
	}
	return f(session)
}

func (s *SystemdSSH) Info(ctx context.Context) (info *enginetypes.Info, err error) {
	cpu, err := s.CPUInfo(ctx)
	if err != nil {
		return
	}
	memory, err := s.MemoryInfo(ctx)
	if err != nil {
		return
	}

	return &enginetypes.Info{
		NCPU:     cpu,
		MemTotal: memory,
	}, nil
}

func (s *SystemdSSH) CPUInfo(ctx context.Context) (cpu int, err error) {
	stdout, stderr, err := s.runSingleCommand(ctx, cmdInspectCPUNumber, nil)
	if err != nil {
		return 0, errors.Wrap(err, stderr.String())
	}
	return strconv.Atoi(strings.TrimSpace(stdout.String()))
}

func (s *SystemdSSH) MemoryInfo(ctx context.Context) (memoryTotalInBytes int64, err error) {
	stdout, stderr, err := s.runSingleCommand(ctx, cmdInspectMemoryTotalInBytes, nil)
	if err != nil {
		return 0, errors.Wrap(err, stderr.String())
	}
	memory, err := strconv.Atoi(strings.TrimSpace(stdout.String()))
	return int64(memory), err
}

func (s *SystemdSSH) ResourceValidate(ctx context.Context, cpu float64, cpumap map[string]int64, memory, storage int64) (err error) {
	return engine.NotImplementedError
}

func (s *SystemdSSH) runSingleCommand(ctx context.Context, cmd string, stdin io.Reader) (stdout, stderr *bytes.Buffer, err error) {
	// what a pathetic library that leaves context completely useless

	stdout = &bytes.Buffer{}
	stderr = &bytes.Buffer{}
	return stdout, stderr, s.WithSession(func(session *ssh.Session) error {
		session.Stdin = stdin
		session.Stdout = stdout
		session.Stderr = stderr
		return session.Run(cmd)
	})
}
