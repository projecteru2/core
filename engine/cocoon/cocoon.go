package cocoon

import (
	"cmp"
	"context"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/cockroachdb/errors"

	"github.com/projecteru2/core/engine"
	"github.com/projecteru2/core/engine/sshrunner"
	enginetypes "github.com/projecteru2/core/engine/types"
	coretypes "github.com/projecteru2/core/types"
)

const (
	// Prefix is the node endpoint scheme this engine serves.
	Prefix = "cocoon://"
	Type   = "cocoon"

	defaultBinary       = "cocoon"
	defaultRoot         = "/var/lib/eru/cocoon"
	defaultRunDir       = "/var/lib/cocoon/run"
	defaultCgroupParent = "cocoon.slice"
	metaDir             = "/run/eru/workloads"
	kiB                 = 1024
	infoFields          = 4

	infoScript = `printf '%s\n' "$(cat /etc/machine-id 2>/dev/null)" "$(nproc 2>/dev/null)" ` +
		`"$(awk '/^MemTotal:/{print $2}' /proc/meminfo 2>/dev/null)" "$(df -Pk "$1" 2>/dev/null | awk 'NR==2{print $2}')"`
)

var _ engine.API = (*Engine)(nil)

// Engine runs VMs through the cocoon CLI over SSH.
type Engine struct {
	config coretypes.Config
	cocoon coretypes.CocoonConfig
	ep     *enginetypes.Params
	runner sshrunner.Runner

	mu    sync.Mutex
	execs map[string]sshrunner.Session

	probe   sync.Mutex
	hasOras bool
}

func MakeClient(_ context.Context, config coretypes.Config, nodename, endpoint, ca, cert, key string) (engine.API, error) {
	user, _, addr, err := sshrunner.ParseEndpoint(endpoint, Prefix)
	if err != nil {
		return nil, err
	}
	clientConfig, err := sshrunner.NewClientConfig(config.SSH, cmp.Or(user, config.SSH.User), config.ConnectionTimeout)
	if err != nil {
		return nil, err
	}
	return &Engine{
		config: config,
		cocoon: coretypes.CocoonConfig{
			Binary:       cmp.Or(config.Cocoon.Binary, defaultBinary),
			Root:         cmp.Or(config.Cocoon.Root, defaultRoot),
			RunDir:       cmp.Or(config.Cocoon.RunDir, defaultRunDir),
			CgroupParent: cmp.Or(config.Cocoon.CgroupParent, defaultCgroupParent),
		},
		ep:     enginetypes.NewParams(nodename, endpoint, ca, cert, key),
		runner: sshrunner.New(addr, clientConfig),
		execs:  map[string]sshrunner.Session{},
	}, nil
}

func (e *Engine) Info(ctx context.Context) (*enginetypes.Info, error) {
	res, err := e.run(ctx, sshrunner.Shell(infoScript, e.cocoon.RunDir)...)
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
	_, err := e.run(ctx, e.cocoon.Binary, "version")
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

// vm renders a cocoon vm subcommand under the node-side binary.
func (e *Engine) vm(args ...string) []string {
	return slices.Concat([]string{e.cocoon.Binary, "vm"}, args)
}
