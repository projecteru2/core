package containerd

import (
	"cmp"
	"context"
	"net"
	"strconv"
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

	kiB        = 1024
	infoFields = 4

	infoScript = `printf '%s\n' "$(cat /etc/machine-id 2>/dev/null)" "$(nproc 2>/dev/null)" ` +
		`"$(awk '/^MemTotal:/{print $2}' /proc/meminfo 2>/dev/null)" "$(df -Pk "$1" 2>/dev/null | awk 'NR==2{print $2}')"`
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

	mu    sync.Mutex
	execs map[string]sshrunner.Session
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
	// containerd serves only the CRI plugin on TCP, so the native API is reached
	// through an OpenSSH socket forward of the node's own unix socket.
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
	return &Engine{
		client:    cli,
		runner:    runner,
		config:    config,
		ep:        enginetypes.NewParams(nodename, endpoint, ca, cert, key),
		namespace: namespace,
		socket:    socket,
		host:      host,
		platform:  platform,
		execs:     map[string]sshrunner.Session{},
	}, nil
}

func (e *Engine) Info(ctx context.Context) (*enginetypes.Info, error) {
	res, err := e.run(ctx, sshrunner.Shell(infoScript, daemonRoot)...)
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

// nodePlatform is what the client matches an image's manifests against; core's own
// platform is not the node's.
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
