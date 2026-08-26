# core

Eru core is a stateless gRPC resource scheduler: it holds cluster metadata in etcd or redis, allocates
resources through pluggable resource plugins, and deploys workloads onto containerd containers,
bare processes or cocoon virtual machines through a single `CoreRPC` API.

**Documentation: [projecteru2.github.io/core](https://projecteru2.github.io/core/)** (source in [`docs/`](docs/)).

[![test](https://github.com/projecteru2/core/actions/workflows/test.yml/badge.svg)](https://github.com/projecteru2/core/actions/workflows/test.yml)
[![lint](https://github.com/projecteru2/core/actions/workflows/lint.yml/badge.svg)](https://github.com/projecteru2/core/actions/workflows/lint.yml)

## Highlights

- **One gRPC API** — the `CoreRPC` service covers pods, nodes, workloads, images, networks, files
  and status streams; long-running calls (deploy, build, logs, exec) are server streams
- **Stateless, multi-instance** — every instance keeps its state in etcd or redis and coordinates
  through distributed locks, so instances can be added and removed freely
- **Multiple engines** — containerd over an SSH-forwarded socket (`containerd://`), cocoon VMs over
  SSH (`cocoon://`), bare processes as systemd transient units over SSH (`process://`) and a mock
  engine, selected per node by endpoint scheme
- **Resource plugins** — `cpumem` is built in; external plugins are ordinary executables in
  `resource_plugin.dir`, invoked with a subcommand and JSON on stdin
  (see [resource-extend](https://github.com/projecteru2/resource-extend) for gpu and storage)
- **Deployment strategies** — `AUTO`, `FILL`, `EACH`, `GLOBAL` and `DRAINED` decide how a deploy
  count is spread over the candidate nodes
- **WAL-based recovery** — allocations, workload creation and processing counters are journaled
  into the store and replayed on start, and a live instance takes over the journal of a dead one,
  so a crash mid-deploy does not leak resources
- **Service discovery + client library** — instances register themselves in the store and push the
  live address list over `WatchServiceStatus`; the Go client in [`client/`](client/) consumes it
- **Embedded etcd** — `--embedded-storage` runs a single-member in-process etcd for a dev instance

## Quick start

```shell
make build

# with an external etcd
./eru-core --config core.yaml.sample

# or a self-contained dev instance (in-process etcd under $TMPDIR/eru-core-etcd)
./eru-core --config core.yaml.sample --embedded-storage
```

The config path also comes from `ERU_CONFIG_PATH`:

```shell
export ERU_CONFIG_PATH=/path/to/core.yaml
eru-core
```

Container image — `ghcr.io/projecteru2/core` (also published as `projecteru2/core`):

```shell
docker run -d \
  --name eru_core_$HOSTNAME \
  --net host \
  --restart always \
  -v <HOST_CONFIG_DIR_PATH>:/etc/eru \
  ghcr.io/projecteru2/core \
  /usr/bin/eru-core
```

See [Installation](docs/installation.md) and [Configuration](docs/configuration.md) for the rest.

## Related projects

- [agent](https://github.com/projecteru2/agent) — per-node daemon that reports node and workload status back to core
- [cli](https://github.com/projecteru2/cli) — command line client for the core API
- [resource-extend](https://github.com/projecteru2/resource-extend) — external resource plugins (gpu, storage)
- [quickstart](https://github.com/projecteru2/quickstart) — a local Eru stack to try things against
- [footstone](https://github.com/projecteru2/footstone) — shared base images

## Development

```shell
make build    # build the eru-core binary (CGO_ENABLED=0)
make test     # go vet on linux+darwin, then tests with -race and coverage
make lint     # golangci-lint on linux+darwin
make fmt      # gofumpt + goimports
make mock     # regenerate mocks from .mockery.yml
make grpc     # regenerate gRPC bindings from rpc/gen/core.proto
make all      # deps, fmt, lint, test, build
```

`make help` lists every target.

## License

This project is licensed under the MIT License. See [`LICENSE`](./LICENSE).
