package cocoon

import (
	"cmp"
	"context"
	"slices"
	"sync/atomic"

	"github.com/cockroachdb/errors"

	"golang.org/x/sync/singleflight"

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
)

var _ engine.API = (*Engine)(nil)

// Engine runs VMs through the cocoon CLI over SSH.
type Engine struct {
	config coretypes.Config
	cocoon coretypes.CocoonConfig
	ep     *enginetypes.Params
	runner sshrunner.Runner

	execs *sshrunner.Execs

	probe   singleflight.Group
	hasOras atomic.Bool
}

func MakeClient(_ context.Context, config coretypes.Config, nodename, endpoint string) (engine.API, error) {
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
		ep:     enginetypes.NewParams(nodename, endpoint),
		runner: sshrunner.New(addr, clientConfig),
		execs:  sshrunner.NewExecs(),
	}, nil
}

func (e *Engine) Info(ctx context.Context) (*enginetypes.Info, error) {
	info, err := sshrunner.NodeInfo(ctx, e.runner, e.cocoon.RunDir)
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
	if _, err := e.run(ctx, e.cocoon.Binary, "version"); err != nil {
		return errors.Wrapf(err, "node %s cannot serve %s", e.ep.Nodename, e.cocoon.Binary)
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

func (e *Engine) vm(args ...string) []string {
	return slices.Concat([]string{e.cocoon.Binary, "vm"}, args)
}
