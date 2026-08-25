package containerd

import (
	"cmp"
	"context"
	"net"
	"strings"
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/containerd/containerd/v2/client"
	cerrdefs "github.com/containerd/errdefs"
	"github.com/containerd/platforms"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/projecteru2/core/engine"
	"github.com/projecteru2/core/engine/sshrunner"
	enginetypes "github.com/projecteru2/core/engine/types"
	coretypes "github.com/projecteru2/core/types"
)

const (
	// Prefix is the node endpoint scheme this engine serves.
	Prefix = "containerd://"
	Type   = "containerd"

	defaultSocket    = "/run/containerd/containerd.sock"
	defaultNamespace = "eru"
	daemonRoot       = "/var/lib/containerd"
	workloadRoot     = "/var/lib/eru/containerd"

	// networkLabelPrefix is where the CNI hook records an attached network's address.
	networkLabelPrefix = "eru.network."
)

// machines maps what uname reports onto the architecture an OCI descriptor names.
var machines = map[string]string{
	"x86_64":  "amd64",
	"aarch64": "arm64",
	"armv7l":  "arm",
}

var _ engine.API = (*Engine)(nil)

// Engine runs containers through a node's own containerd socket, forwarded over SSH.
type Engine struct {
	client    *client.Client
	runner    sshrunner.Runner
	config    coretypes.Config
	ep        *enginetypes.Params
	namespace string
	socket    string
	host      string
	platform  ocispec.Platform

	execs *sshrunner.Execs

	mu       sync.Mutex
	attaches map[string]*attach
}

// MakeClient builds a containerd engine for endpoint.
func MakeClient(ctx context.Context, config coretypes.Config, nodename, endpoint, ca, cert, key string) (engine.API, error) {
	user, host, addr, err := sshrunner.ParseEndpoint(endpoint, Prefix)
	if err != nil {
		return nil, err
	}
	clientConfig, err := sshrunner.NewClientConfig(config.SSH, cmp.Or(user, config.SSH.User), config.ConnectionTimeout)
	if err != nil {
		return nil, err
	}

	runner := sshrunner.New(addr, clientConfig)
	platform, err := nodePlatform(ctx, runner)
	if err != nil {
		_ = runner.Close()
		return nil, err
	}
	socket := cmp.Or(config.Containerd.Socket, defaultSocket)
	namespace := cmp.Or(config.Containerd.Namespace, defaultNamespace)
	// containerd serves only CRI on TCP, so the native API rides an ssh socket forward
	cli, err := client.New(socket,
		client.WithDefaultNamespace(namespace),
		client.WithDefaultPlatform(platforms.Only(platform)),
		client.WithTimeout(config.ConnectionTimeout),
		client.WithDialOpts([]grpc.DialOption{
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
				return runner.Dial(ctx, "unix", socket)
			}),
		}),
	)
	if err != nil {
		_ = runner.Close()
		return nil, err
	}
	return newEngine(&Engine{
		client:    cli,
		runner:    runner,
		config:    config,
		ep:        enginetypes.NewParams(nodename, endpoint, ca, cert, key),
		namespace: namespace,
		socket:    socket,
		host:      host,
		platform:  platform,
	}), nil
}

func newEngine(e *Engine) *Engine {
	e.execs = sshrunner.NewExecs()
	e.attaches = map[string]*attach{}
	return e
}

func (e *Engine) Info(ctx context.Context) (*enginetypes.Info, error) {
	info, err := sshrunner.NodeInfo(ctx, e.runner, daemonRoot)
	if err != nil {
		return nil, err
	}
	info.Type = Type
	return info, nil
}

func (e *Engine) Ping(ctx context.Context) error {
	serving, err := e.client.IsServing(ctx)
	if err != nil {
		return err
	}
	if !serving {
		return errors.Newf("containerd on %s is not serving", e.ep.Nodename)
	}
	return nil
}

func (e *Engine) CloseConn() error {
	return errors.Join(e.client.Close(), e.runner.Close())
}

func (e *Engine) GetParams() *enginetypes.Params {
	return e.ep
}

func (e *Engine) RawEngine(context.Context, *enginetypes.RawEngineOptions) (*enginetypes.RawEngineResult, error) {
	return nil, coretypes.ErrEngineNotImplemented
}

func (e *Engine) container(ctx context.Context, ID string) (client.Container, error) {
	found, err := e.client.LoadContainer(ctx, ID)
	if err != nil && (cerrdefs.IsNotFound(err) || cerrdefs.IsInvalidArgument(err)) {
		return nil, errors.Wrapf(coretypes.ErrWorkloadNotExists, "no workload %s", ID)
	}
	return found, err
}

// run executes argv on the node; the node answers what containerd's API does not carry.
func (e *Engine) run(ctx context.Context, argv ...string) (*sshrunner.Result, error) {
	res, err := e.runner.Run(ctx, sshrunner.Quote(argv), nil)
	if err != nil {
		return nil, err
	}
	return res, sshrunner.ExitError(argv, res)
}

// nodePlatform is what the client matches manifests against; core's platform is not the node's.
func nodePlatform(ctx context.Context, runner sshrunner.Runner) (ocispec.Platform, error) {
	res, err := runner.Run(ctx, sshrunner.Quote([]string{"uname", "-m"}), nil)
	if err != nil {
		return ocispec.Platform{}, err
	}
	machine := strings.TrimSpace(res.Stdout)
	arch, ok := machines[machine]
	if !ok {
		return ocispec.Platform{}, errors.Wrapf(coretypes.ErrInvaildNodeEndpoint, "unsupported node machine %q", machine)
	}
	return ocispec.Platform{OS: "linux", Architecture: arch}, nil
}
