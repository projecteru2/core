# Installation

Release archives, the container image, building from source, and running core as a service.

## Requirements

- An etcd cluster (default) or a redis server for metadata — see [Storage](storage.md).
  A single dev instance can use the in-process etcd instead (`--embedded-storage`).
- Reachable engine endpoints on the nodes core manages — see [Engines](engines.md).
- Go 1.27+ to build from source.

## GitHub Releases

Tagged releases are built by goreleaser for `linux` and `darwin`, `amd64` and `arm64`.
Archives are named `core_<version>_<Os>_<Arch>.tar.gz` and contain the `eru-core` binary
plus `LICENSE` and `README.md`:

```shell
# Linux amd64, a specific tag
curl -fsSL -o core.tar.gz \
  https://github.com/projecteru2/core/releases/download/v23.10.11/core_23.10.11_Linux_x86_64.tar.gz

tar -xzf core.tar.gz
install -m 0755 eru-core /usr/bin/
```

Every release also publishes unversioned aliases, so `releases/latest/download` resolves without
knowing the tag:

```shell
curl -fsSL -o core.tar.gz \
  https://github.com/projecteru2/core/releases/latest/download/core_Linux_x86_64.tar.gz
```

| Artifact | Contents |
| --- | --- |
| `core_<ver>_Linux_x86_64.tar.gz`, `core_<ver>_Linux_arm64.tar.gz` | stripped `eru-core` for Linux |
| `core_<ver>_Darwin_x86_64.tar.gz`, `core_<ver>_Darwin_arm64.tar.gz` | stripped `eru-core` for macOS |
| `core_<ver>_Linux_x86_64_debug.tar.gz`, `core_<ver>_Linux_arm64_debug.tar.gz` | unstripped `eru-core.dbg`, Linux only |
| `core_<Os>_<Arch>.tar.gz` | unversioned alias of the release build |
| `checksums.txt` | SHA-256 of every archive |

## Container image

Images are published on every `master` push and every `v*` tag, to both GHCR and Docker Hub,
for `linux/amd64` and `linux/arm64`:

```shell
docker pull ghcr.io/projecteru2/core:v23.10.11   # or projecteru2/core:v23.10.11
```

The image is Alpine-based, ships the binary at `/usr/bin/eru-core` and a copy of the sample
config at `/etc/eru/core.yaml.sample`. Mount your own config directory over `/etc/eru`:

```shell
docker run -d \
  --name eru_core_$HOSTNAME \
  --net host \
  --restart always \
  -v <HOST_CONFIG_DIR_PATH>:/etc/eru \
  ghcr.io/projecteru2/core \
  /usr/bin/eru-core
```

`--net host` matters: core registers its own outbound address for service discovery, and it needs
to reach every node's engine endpoint.

## Build from source

```shell
git clone https://github.com/projecteru2/core.git
cd core
make build
```

This produces `eru-core` in the repo root, built with `CGO_ENABLED=0` and the version, revision
and build time baked in through ldflags. `KEEP_SYMBOL=1 make build` keeps the symbol table.

To regenerate the gRPC bindings after editing `rpc/gen/core.proto` you also need `protoc` on
`PATH`; `make grpc` installs the pinned `protoc-gen-go` and `protoc-gen-go-grpc` into `bin/`.

## Running

Core takes two flags of its own, plus what urfave/cli adds:

| Flag | Default | Meaning |
| --- | --- | --- |
| `--config` | `/etc/eru/core.yaml` | config file path, in YAML. Also read from `ERU_CONFIG_PATH` |
| `--embedded-storage` | off | start a single-member in-process etcd under `$TMPDIR/eru-core-etcd` and use it as the store |
| `--version` | | print version, git hash, build time, Go version and OS/arch |

```shell
eru-core --config /etc/eru/core.yaml
```

```shell
export ERU_CONFIG_PATH=/path/to/core.yaml
eru-core
```

`--embedded-storage` is for development and tests only — the data lives in a temp directory and
no other process can reach it. The rest of the config is still read normally; see
[Configuration](configuration.md).

## systemd

The repo ships [`eru-core.service`](https://github.com/projecteru2/core/blob/master/eru-core.service),
which runs `/usr/bin/eru-core --config /etc/eru/core.yaml`:

```shell
install -m 0644 eru-core.service /usr/lib/systemd/system/
install -m 0644 core.yaml.sample /etc/eru/core.yaml   # then edit it
systemctl daemon-reload
systemctl enable --now eru-core
```

The unit raises `LimitNOFILE`/`LimitNPROC` to 10485760, allows core dumps
(`LimitCORE=infinity`, `GOTRACEBACK=crash`) and gives a 1200s stop timeout so in-flight
streaming tasks can drain — core stops its gRPC server gracefully and waits for running tasks
before exiting.
