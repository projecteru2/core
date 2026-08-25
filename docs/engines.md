# Engines

An engine is core's client for one node's runtime. `engine.API` covers the whole surface core
needs: virtualization lifecycle (create, start, stop, remove, suspend, resume, inspect, wait,
attach, resize, logs, update resource, copy in/out), exec, image operations, networks, node info
and a passthrough `RawEngine` call.

Core never chooses an engine globally — it chooses one per node, from that node's endpoint.

## Endpoint schemes

The scheme prefix of `AddNode`'s `endpoint` selects the implementation:

| Prefix | Implementation | Notes |
| --- | --- | --- |
| `tcp://` | `engine/docker` | Docker daemon over TCP, optionally TLS |
| `unix://` | `engine/docker` | Docker daemon over a local socket |
| `virt-grpc://` | `engine/virt` | yavirt, over the libyavirt gRPC client |
| `systemd://` | `engine/systemd` | Docker client wrapped to force a containerd runtime |
| `mock://` | `engine/mocks/fakeengine` | Fully mocked engine, for tests and dry runs |

An endpoint with any other prefix is rejected with `ErrInvaildNodeEndpoint`.

## docker

The default. Uses the moby client (`github.com/moby/moby/client`), pinning `docker.version` as the API
version, which disables API-version negotiation.

- Network mode comes from the deploy request, falling back to `docker.network_mode`.
- If a deploy specifies no DNS and `docker.use_local_dns` is on, the node's own IP is injected as
  the workload's resolver.
- Log driver: core forces `mode=non-blocking`, `max-buffer-size=4m` and a per-workload `tag`, then
  merges `docker.log.config`; the driver itself is the entrypoint's `log.type`, or
  `docker.log.type`.
- Registry credentials for pull, push and build come from `docker.auths`, matched by registry host.
- Image builds happen here — `BuildRefs` composes `hub/namespace/appname:tag` from
  `docker.hub` and `docker.namespace`.

### TLS

`AddNode` and `SetNode` accept `ca`, `cert` and `key` as *inline PEM content*, not paths. Core
writes them to temporary files under `cert_path`, builds the TLS config, and deletes the temp
files immediately; the resulting HTTP client is cached. If `cert_path` is empty, or any of the
three is missing, core falls back to a plain HTTP client. The PEM material is stored per node in
the metadata store, under `/node/<nodename>:ca`, `:cert` and `:key`.

`unix://` endpoints always use the local socket client and ignore TLS.

## virt (yavirt)

Talks to [yavirt](https://github.com/projecteru2/yavirt) through
[libyavirt](https://github.com/projecteru2/libyavirt); `virt-grpc://host:port` is rewritten to
`grpc://host:port` for the client. `virt.version` selects the yavirtd API version. Only the `ca`
field is used, written to a temp file under `cert_path`.

This is the engine that implements `RawEngine`: `op` and `params` are forwarded verbatim to
yavirt, so VM-specific operations reachable through core need no proto change.

Node info from yavirt carries a `resources` map, which the resource plugins read when the node is
added — this is how a VM node reports its real CPU, memory and storage.

## systemd

Embeds the docker engine and overrides a few methods:

- `VirtualizationCreate` forces `runtime` to `systemd.runtime` (default `io.containerd.eru.v2`)
  in the raw args before delegating.
- Exec, attach, resize, logs, wait, resource update, network operations and image build all
  return `ErrEngineNotImplemented`.

So a `systemd://` node can deploy and remove workloads, but not exec into them, stream their logs
or take part in network or build operations.

## fake

Two different things share the name:

- `engine/fake.EngineWithErr` — a placeholder every method of which returns one stored error. The
  engine cache substitutes it for an engine that has stopped responding, so callers get the real
  connection error instead of a nil dereference.
- `engine/mocks/fakeengine` (`mock://`) — a testify-based mock that reports a 100-CPU,
  100 GiB node and accepts every operation. Used by tests and for dry-running scheduling logic.

## The engine cache

`engine/factory` keeps one client per `(endpoint, ca, cert, key)` tuple, so repeated calls to the
same node reuse one connection. Two background loops keep it honest:

- **Liveness sweep** — every `connection_timeout`, `Ping` every cached engine. A failing engine is
  replaced by `EngineWithErr` holding the error; a cached `EngineWithErr` is retried, and if it
  connects, the real client takes its place. If the retry fails and the node's status key is gone,
  the entry is dropped.
- **Node status subscriber** — reads the store's node status stream. When a node goes down, every
  cached engine belonging to it is evicted; when a node's metadata turns out to be invalid, its
  metrics labels are removed too.

`GetEngine` caches failures as well as successes, deliberately: a node that cannot be reached
gets a fast, explanatory error rather than a connect timeout on every call.
