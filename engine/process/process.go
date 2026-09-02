package process

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/cockroachdb/errors"

	"github.com/projecteru2/core/engine"
	"github.com/projecteru2/core/engine/sshrunner"
	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/engine/workloadmeta"
	coretypes "github.com/projecteru2/core/types"
)

const (
	// Prefix is the node endpoint scheme this engine serves.
	Prefix = "process://"
	Type   = "process"

	defaultRoot        = "/var/lib/eru/process"
	defaultStopTimeout = 10 * time.Second
	hostNetwork        = "host"
)

var metaScript = fmt.Sprintf(`dir=$1
test -f "$dir/meta.json" || exit %d
if mountpoint -q "$dir/merged"; then echo 1; else echo 0; fi
cat "$dir/meta.json"
`, workloadmeta.NotExistsCode)

var _ engine.API = (*Engine)(nil)

// Engine runs bare processes as systemd transient units over SSH.
type Engine struct {
	config      coretypes.Config
	ep          *enginetypes.Params
	runner      sshrunner.Runner
	root        string
	host        string
	stopTimeout time.Duration
	execs       *sshrunner.Execs
}

func MakeClient(_ context.Context, config coretypes.Config, nodename, endpoint string) (engine.API, error) {
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
		ep:          enginetypes.NewParams(nodename, endpoint),
		runner:      sshrunner.New(addr, clientConfig),
		root:        cmp.Or(config.Process.Root, defaultRoot),
		host:        host,
		stopTimeout: cmp.Or(config.Process.StopTimeout, defaultStopTimeout),
		execs:       sshrunner.NewExecs(),
	}, nil
}

func (e *Engine) Info(ctx context.Context) (*enginetypes.Info, error) {
	info, err := sshrunner.NodeInfo(ctx, e.runner, e.root)
	if err != nil {
		return nil, err
	}
	info.Type = Type
	return info, nil
}

func (e *Engine) Ping(ctx context.Context) error {
	return e.runner.Ping(ctx)
}

func (e *Engine) VerifyNode(ctx context.Context) error {
	if _, err := e.run(ctx, "systemctl", "--version"); err != nil {
		return errors.Wrapf(err, "node %s cannot serve systemd", e.ep.Nodename)
	}
	return nil
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

func (e *Engine) call(ctx context.Context, argv ...string) (*sshrunner.Result, error) {
	return sshrunner.Call(ctx, e.runner, argv...)
}

func (e *Engine) run(ctx context.Context, argv ...string) (*sshrunner.Result, error) {
	return sshrunner.Run(ctx, e.runner, argv...)
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
