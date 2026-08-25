package process

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cockroachdb/errors"

	"github.com/projecteru2/core/engine"
	"github.com/projecteru2/core/engine/sshrunner"
	enginetypes "github.com/projecteru2/core/engine/types"
	coretypes "github.com/projecteru2/core/types"
)

const (
	// Prefix is the node endpoint scheme this engine serves.
	Prefix = "process://"
	Type   = "process"

	defaultRoot        = "/var/lib/eru/process"
	defaultStopTimeout = 10 * time.Second
	metaDir            = "/run/eru/workloads"
	hostNetwork        = "host"
	kiB                = 1024
	infoFields         = 4

	infoScript = `mkdir -p "$1" 2>/dev/null || true
printf '%s\n' "$(cat /etc/machine-id 2>/dev/null)" "$(nproc 2>/dev/null)" ` +
		`"$(awk '/^MemTotal:/{print $2}' /proc/meminfo 2>/dev/null)" "$(df -Pk "$1" 2>/dev/null | awk 'NR==2{print $2}')"`
)

var metaScript = fmt.Sprintf(`dir=$1
test -f "$dir/meta.json" || exit %d
if mountpoint -q "$dir/merged"; then echo 1; else echo 0; fi
cat "$dir/meta.json"
`, notExistsCode)

var _ engine.API = (*Engine)(nil)

// Engine runs bare processes as systemd transient units over SSH.
type Engine struct {
	config      coretypes.Config
	ep          *enginetypes.Params
	runner      sshrunner.Runner
	root        string
	host        string
	stopTimeout time.Duration

	mu    sync.Mutex
	execs map[string]sshrunner.Session
}

func MakeClient(_ context.Context, config coretypes.Config, nodename, endpoint, ca, cert, key string) (engine.API, error) {
	user, host, addr, err := sshrunner.ParseEndpoint(endpoint, Prefix)
	if err != nil {
		return nil, err
	}
	clientConfig, err := sshrunner.NewClientConfig(config.SSH, cmp.Or(user, config.SSH.User), config.ConnectionTimeout)
	if err != nil {
		return nil, err
	}
	return &Engine{
		config:      config,
		ep:          enginetypes.NewParams(nodename, endpoint, ca, cert, key),
		runner:      sshrunner.New(addr, clientConfig),
		root:        cmp.Or(config.Process.Root, defaultRoot),
		host:        host,
		stopTimeout: cmp.Or(config.Process.StopTimeout, defaultStopTimeout),
		execs:       map[string]sshrunner.Session{},
	}, nil
}

func (e *Engine) Info(ctx context.Context) (*enginetypes.Info, error) {
	res, err := e.run(ctx, sshrunner.Shell(infoScript, e.root)...)
	if err != nil {
		return nil, err
	}
	fields := strings.Split(strings.TrimRight(res.Stdout, "\n"), "\n")
	if len(fields) < infoFields {
		return nil, errors.Wrapf(coretypes.ErrInvaildNodeEndpoint, "unexpected node info %q", res.Stdout)
	}
	ncpu, _ := strconv.Atoi(fields[1])
	memory, _ := strconv.ParseInt(fields[2], 10, 64)
	storage, _ := strconv.ParseInt(fields[3], 10, 64)
	return &enginetypes.Info{
		Type:         Type,
		ID:           fields[0],
		NCPU:         ncpu,
		MemTotal:     memory * kiB,
		StorageTotal: storage * kiB,
	}, nil
}

func (e *Engine) Ping(ctx context.Context) error {
	_, err := e.run(ctx, "systemctl", "--version")
	return err
}

func (e *Engine) CloseConn() error {
	return e.runner.Close()
}

func (e *Engine) GetParams() *enginetypes.Params {
	return e.ep
}

func (e *Engine) RawEngine(context.Context, *enginetypes.RawEngineOptions) (*enginetypes.RawEngineResult, error) {
	return nil, coretypes.ErrEngineNotImplemented
}

// call runs argv on the node; a non-zero exit is reported in the result, not as an error.
func (e *Engine) call(ctx context.Context, argv ...string) (*sshrunner.Result, error) {
	return e.runner.Run(ctx, sshrunner.Quote(argv), nil)
}

func (e *Engine) run(ctx context.Context, argv ...string) (*sshrunner.Result, error) {
	res, err := e.call(ctx, argv...)
	if err != nil {
		return nil, err
	}
	return res, sshrunner.ExitError(argv, res)
}

// workloadMeta reads the durable record and the overlay's mount state in one round trip.
func (e *Engine) workloadMeta(ctx context.Context, ID string) (*meta, bool, error) {
	res, err := e.call(ctx, sshrunner.Shell(metaScript, workloadDir(e.root, ID))...)
	if err != nil {
		return nil, false, err
	}
	if res.Code != 0 {
		return nil, false, errors.Wrapf(coretypes.ErrWorkloadNotExists, "no meta file for %s", ID)
	}
	mounted, body, _ := strings.Cut(res.Stdout, "\n")
	record := &meta{}
	if err = json.Unmarshal([]byte(body), record); err != nil {
		return nil, false, err
	}
	return record, mounted == "1", nil
}
