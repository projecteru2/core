package process

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/projecteru2/core/engine/sshrunner"
	enginetypes "github.com/projecteru2/core/engine/types"
	coretypes "github.com/projecteru2/core/types"
)

const (
	testRoot    = "/var/lib/eru/process"
	overlayMeta = `{"id":"w1","kind":"process","podname":"prod","root_directory":"/var/lib/eru/process/w1/merged"}`
	rawMeta     = `{"id":"w1","kind":"process","podname":"prod","working_dir":"/srv/app"}`
)

type fakeRunner struct {
	lines   []string
	respond func(line string) *sshrunner.Result
	started *fakeSession
}

func (r *fakeRunner) Run(_ context.Context, line string, _ io.Reader) (*sshrunner.Result, error) {
	r.lines = append(r.lines, line)
	if r.respond != nil {
		return r.respond(line), nil
	}
	return &sshrunner.Result{}, nil
}

func (r *fakeRunner) Start(_ context.Context, line string, _ *sshrunner.StartOptions) (sshrunner.Session, error) {
	r.lines = append(r.lines, line)
	return r.started, nil
}

func (r *fakeRunner) Files(context.Context) (sshrunner.Files, error) {
	return nil, coretypes.ErrEngineNotImplemented
}

func (r *fakeRunner) Close() error {
	return nil
}

type fakeSession struct {
	code   int
	stdout string
	closed bool
}

func (s *fakeSession) Stdin() io.WriteCloser {
	return nil
}

func (s *fakeSession) Stdout() io.ReadCloser {
	return io.NopCloser(strings.NewReader(s.stdout))
}

func (s *fakeSession) Stderr() io.ReadCloser {
	return io.NopCloser(strings.NewReader(""))
}

func (s *fakeSession) Resize(uint, uint) error {
	return nil
}

func (s *fakeSession) Wait() (int, error) {
	return s.code, nil
}

func (s *fakeSession) Close() error {
	s.closed = true
	return nil
}

func testEngine(t *testing.T, runner *fakeRunner) *Engine {
	t.Helper()
	return &Engine{
		ep:          enginetypes.NewParams("node1", Prefix+"10.0.0.1", "", "", ""),
		runner:      runner,
		root:        testRoot,
		host:        "10.0.0.1",
		stopTimeout: defaultStopTimeout,
		execs:       map[string]sshrunner.Session{},
	}
}
