package containerd

import (
	"cmp"
	"context"
	"encoding/json"
	"io"
	"net"
	"os"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/docker/cli/cli/config/types"
	bkclient "github.com/moby/buildkit/client"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/session/auth/authprovider"
	"github.com/tonistiigi/fsutil"
	"golang.org/x/sync/errgroup"

	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/log"
	coresource "github.com/projecteru2/core/source"
	coretypes "github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

const (
	defaultBuildKit = "/run/buildkit/buildkitd.sock"
	tcpPrefix       = "tcp://"

	dockerfileFrontend = "dockerfile.v0"
	imageExporter      = "image"
	contextMount       = "context"
	dockerfileMount    = "dockerfile"
)

func (e *Engine) BuildRefs(_ context.Context, opts *enginetypes.BuildRefOptions) []string {
	if len(opts.Tags) == 0 {
		return []string{normalizeRef(e.config.Registry.ImageTag(opts.Name, utils.DefaultVersion))}
	}
	refs := make([]string, 0, len(opts.Tags))
	for _, tag := range opts.Tags {
		refs = append(refs, normalizeRef(e.config.Registry.ImageTag(opts.Name, tag)))
	}
	return refs
}

// BuildContent renders the build spec into a Dockerfile and tars the context around it.
// Layout: <buildDir>/<reponame>/<code> next to <buildDir>/Dockerfile.
func (e *Engine) BuildContent(ctx context.Context, scm coresource.Source, opts *enginetypes.BuildContentOptions) (string, io.Reader, error) {
	if opts.Builds == nil {
		return "", nil, coretypes.ErrNoBuildsInSpec
	}
	buildDir, err := os.MkdirTemp(os.TempDir(), "corebuild-")
	if err != nil {
		return "", nil, err
	}
	log.WithFunc("engine.containerd.BuildContent").Debugf(ctx, "build dir %s", buildDir)
	if err = makeDockerfile(ctx, opts, scm, buildDir); err != nil {
		return buildDir, nil, err
	}
	tar, err := createTarStream(buildDir)
	return buildDir, tar, err
}

// ImageBuild solves the context on the node's buildkitd and exports straight to the registry.
func (e *Engine) ImageBuild(ctx context.Context, input io.Reader, refs []string, platform string) (io.ReadCloser, error) {
	dir, err := os.MkdirTemp(os.TempDir(), "erusolve-")
	if err != nil {
		return nil, err
	}
	if err = unpackContext(input, dir); err != nil {
		return nil, errors.Join(err, os.RemoveAll(dir))
	}
	contextFS, err := fsutil.NewFS(dir)
	if err != nil {
		return nil, errors.Join(err, os.RemoveAll(dir))
	}
	buildkit, err := e.buildkit(ctx)
	if err != nil {
		return nil, errors.Join(err, os.RemoveAll(dir))
	}

	opt := bkclient.SolveOpt{
		Frontend:      dockerfileFrontend,
		FrontendAttrs: frontendAttrs(platform),
		LocalMounts:   map[string]fsutil.FS{contextMount: contextFS, dockerfileMount: contextFS},
		Exports: []bkclient.ExportEntry{{
			Type:  imageExporter,
			Attrs: map[string]string{"name": strings.Join(refs, ","), "push": "true"},
		}},
		Session: []session.Attachable{e.buildAuth()},
	}

	status := make(chan *bkclient.SolveStatus)
	reader, writer := io.Pipe()
	group, solveCtx := errgroup.WithContext(ctx)
	group.Go(func() error {
		_, solveErr := buildkit.Solve(solveCtx, nil, opt, status)
		return solveErr
	})
	group.Go(func() error {
		return streamSolveStatus(status, writer)
	})
	go func() {
		err := group.Wait()
		_ = writeSolveError(writer, err)
		_ = writer.Close()
		_ = buildkit.Close()
		_ = os.RemoveAll(dir)
	}()
	return reader, nil
}

func (e *Engine) ImageBuildCachePrune(ctx context.Context, all bool) (uint64, error) {
	buildkit, err := e.buildkit(ctx)
	if err != nil {
		return 0, err
	}
	defer func() {
		_ = buildkit.Close()
	}()

	usage := make(chan bkclient.UsageInfo)
	reclaimed := int64(0)
	group, pruneCtx := errgroup.WithContext(ctx)
	group.Go(func() error {
		defer close(usage)
		return buildkit.Prune(pruneCtx, usage, pruneOpts(all)...)
	})
	group.Go(func() error {
		for entry := range usage {
			reclaimed += entry.Size
		}
		return nil
	})
	if err = group.Wait(); err != nil {
		return 0, err
	}
	return uint64(max(reclaimed, 0)), nil //nolint:gosec // a negative sum of blob sizes is not reachable
}

// buildkit dials buildkitd through the node's SSH connection unless it is given a TCP address.
func (e *Engine) buildkit(ctx context.Context) (*bkclient.Client, error) {
	address := cmp.Or(e.config.Containerd.BuildKit, defaultBuildKit)
	if strings.HasPrefix(address, tcpPrefix) {
		return bkclient.New(ctx, address)
	}
	return bkclient.New(ctx, "unix://"+address, bkclient.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
		return e.runner.Dial(ctx, "unix", address)
	}))
}

// buildAuth hands core's registry credentials to the solve session.
func (e *Engine) buildAuth() session.Attachable {
	auths := e.config.Registry.Auths
	return authprovider.NewDockerAuthProvider(authprovider.DockerAuthProviderConfig{
		AuthConfigProvider: func(_ context.Context, host string, _ []string, _ authprovider.ExpireCachedAuthCheck) (types.AuthConfig, error) {
			if host == authprovider.DockerHubRegistryHost {
				host = authprovider.DockerHubConfigfileKey
			}
			auth, ok := auths[host]
			if !ok {
				return types.AuthConfig{}, nil
			}
			return types.AuthConfig{ServerAddress: host, Username: auth.Username, Password: auth.Password}, nil
		},
	})
}

func frontendAttrs(platform string) map[string]string {
	attrs := map[string]string{"filename": "Dockerfile", "no-cache": "true"}
	if platform != "" {
		attrs["platform"] = platform
	}
	return attrs
}

func pruneOpts(all bool) []bkclient.PruneOption {
	if all {
		return []bkclient.PruneOption{bkclient.PruneAll}
	}
	return nil
}

// streamSolveStatus renders buildkit's graph as the build messages core streams to clients.
func streamSolveStatus(status <-chan *bkclient.SolveStatus, out io.Writer) error {
	encoder := json.NewEncoder(out)
	for update := range status {
		for _, vertex := range update.Vertexes {
			message := &coretypes.BuildImageMessage{Stream: vertex.Name + "\n", Status: vertexStatus(vertex)}
			if vertex.Error != "" {
				message.Error = vertex.Error
				message.ErrorDetail.Message = vertex.Error
			}
			if err := encoder.Encode(message); err != nil {
				return err
			}
		}
		for _, entry := range update.Logs {
			if err := encoder.Encode(&coretypes.BuildImageMessage{Stream: string(entry.Data)}); err != nil {
				return err
			}
		}
	}
	return nil
}

func writeSolveError(out io.Writer, err error) error {
	if err == nil {
		return nil
	}
	message := &coretypes.BuildImageMessage{Error: err.Error()}
	message.ErrorDetail.Code = -1
	message.ErrorDetail.Message = err.Error()
	return json.NewEncoder(out).Encode(message)
}

func vertexStatus(vertex *bkclient.Vertex) string {
	switch {
	case vertex.Cached:
		return "cached"
	case vertex.Completed != nil:
		return "finished"
	case vertex.Started != nil:
		return "running"
	default:
		return "pending"
	}
}
