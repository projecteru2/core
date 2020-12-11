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
	"github.com/projecteru2/core/types"
	coretypes "github.com/projecteru2/core/types"
	"golang.org/x/crypto/ssh"

	"github.com/projecteru2/core/log"
)

const (
	// SSHPrefixKey is engine endpoint prefix
	SSHPrefixKey = "systemd://"

	cmdInspectCPUNumber          = "/bin/grep -c processor /proc/cpuinfo"
	cmdInspectMemoryTotalInBytes = "/usr/bin/awk '/^Mem/ {print $2}' <(/usr/bin/free -bt)"
)

// SSHClient contains a connection to sshd
type SSHClient struct {
	hostIP string
	client *ssh.Client
}

// NewSSHClient creates a SSHClient pointer
func NewSSHClient(endpoint string, config *ssh.ClientConfig) (*SSHClient, error) {
	parts := strings.Split(endpoint, ":")
	client, err := ssh.Dial("tcp", endpoint, config)
	return &SSHClient{
		hostIP: parts[0],
		client: client,
	}, err
}

// MakeClient makes systemd engine instance
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
	return NewSSHClient(
		strings.TrimPrefix(endpoint, SSHPrefixKey),
		sshConfig,
	)
}

func (s *SSHClient) withSession(f func(*ssh.Session) error) (err error) {
	session, err := s.client.NewSession()
	if err != nil {
		return
	}
	defer session.Close()
	return f(session)
}

// Info fetches cpu info of remote
func (s *SSHClient) Info(ctx context.Context) (info *enginetypes.Info, err error) {
	cpu, err := s.cpuInfo(ctx)
	if err != nil {
		return
	}
	memory, err := s.memoryInfo(ctx)
	if err != nil {
		return
	}

	return &enginetypes.Info{
		NCPU:     cpu,
		MemTotal: memory,
	}, nil
}

func (s *SSHClient) cpuInfo(ctx context.Context) (cpu int, err error) {
	stdout, stderr, err := s.runSingleCommand(ctx, cmdInspectCPUNumber, nil)
	if err != nil {
		return 0, errors.Wrap(err, stderr.String())
	}
	return strconv.Atoi(strings.TrimSpace(stdout.String()))
}

func (s *SSHClient) memoryInfo(ctx context.Context) (memoryTotalInBytes int64, err error) {
	stdout, stderr, err := s.runSingleCommand(ctx, cmdInspectMemoryTotalInBytes, nil)
	if err != nil {
		return 0, errors.Wrap(err, stderr.String())
	}
	memory, err := strconv.Atoi(strings.TrimSpace(stdout.String()))
	return int64(memory), err
}

// ResourceValidate validates resources
func (s *SSHClient) ResourceValidate(ctx context.Context, cpu float64, cpumap map[string]int64, memory, storage int64) (err error) {
	return types.ErrEngineNotImplemented
}

func (s *SSHClient) runSingleCommand(_ context.Context, cmd string, stdin io.Reader) (stdout, stderr *bytes.Buffer, err error) {
	// what a pathetic library that leaves context completely useless
	log.Debugf("[runSingleCommand] %s", cmd)

	stdout = &bytes.Buffer{}
	stderr = &bytes.Buffer{}
	return stdout, stderr, s.withSession(func(session *ssh.Session) error {
		session.Stdin = stdin
		session.Stdout = stdout
		session.Stderr = stderr
		return session.Run(cmd)
	})
}
