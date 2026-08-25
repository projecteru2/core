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
| `virt-grpc://` | `engine/virt` | yavirt (archived), over the libyavirt gRPC client |
| `process://` | `engine/process` | Bare processes as systemd transient units, over SSH |
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
  `docker.hub` and `docker.namespace`. Which nodes may build is decided by `build.node_filter`,
  not by the engine.

### TLS

`AddNode` and `SetNode` accept `ca`, `cert` and `key` as *inline PEM content*, not paths. Core
writes them to temporary files under `cert_path`, builds the TLS config, and deletes the temp
files immediately; the resulting HTTP client is cached. If `cert_path` is empty, or any of the
three is missing, core falls back to a plain HTTP client. The PEM material is stored per node in
the metadata store, under `/node/<nodename>:ca`, `:cert` and `:key`.

`unix://` endpoints always use the local socket client and ignore TLS.

## virt (yavirt)

[yavirt](https://github.com/projecteru2/yavirt) is archived and no longer developed; the engine and
its [libyavirt](https://github.com/projecteru2/libyavirt) dependency still ship and still resolve,
so existing VM nodes keep working. `virt-grpc://host:port` is rewritten to
`grpc://host:port` for the client. `virt.version` selects the yavirtd API version. Only the `ca`
field is used, written to a temp file under `cert_path`.

This is the engine that implements `RawEngine`: `op` and `params` are forwarded verbatim to
yavirt, so VM-specific operations reachable through core need no proto change.

Node info from yavirt carries a `resources` map, which the resource plugins read when the node is
added — this is how a VM node reports its real CPU, memory and storage.

## process

`process://[user@]host[:port]` nodes run bare processes. Core reaches them over SSH with the key
pair in the `ssh` config block — the endpoint's user overrides `ssh.user` — and drives `systemd`,
`journalctl`, `oras` and sftp. No eru daemon runs on the node. Every remote command is built as an
argv and single-quoted before it is sent, so workload names, paths and environment values are
never interpolated into a shell line.

One transient service per workload (`eru-<id>.service`), one slice per pod (`eru-<pod>.slice`):

| `engine.API` | Node command |
| --- | --- |
| `VirtualizationCreate` | `oras pull` the artifact into `<process.root>/<id>/lower`, prepare `upper`, `work` and `merged`, write the meta file, and record the rendered `systemd-run` command in `<process.root>/<id>/run.sh`. Nothing runs yet |
| `VirtualizationStart` | mount the overlay at `merged`, then run `run.sh` |
| `VirtualizationStop` | `systemctl stop`, then unmount; a forced stop sends `SIGKILL` first |
| `VirtualizationRemove` | `systemctl reset-failed`, unmount, delete the workload directory and the meta file |
| `VirtualizationSuspend` / `Resume` | `systemctl freeze` / `thaw` |
| `VirtualizationInspect` | `systemctl show` |
| `VirtualizationWait` | poll `systemctl show -p ActiveState` until the unit is inactive, then return `ExecMainStatus` |
| `VirtualizationLogs` | `journalctl -u eru-<id> -o cat`, with `-f`, `-n`, `--since` and `--until` |
| `VirtualizationAttach` | logs-follow; stdin returns `ErrEngineNotImplemented` |
| `Execute` | `systemd-run --scope` in the workload's slice and root, stdio streamed over the SSH session, exit code from the scope |
| `VirtualizationUpdateResource` | `systemctl set-property` — live, no restart |
| `VirtualizationCopyTo` / `CopyFrom` | sftp into the overlay `upper`, or into the working directory of a raw workload |
| `ImagePull` / `ImageList` / `ImageRemove` | `oras pull` into the artifact cache / list the cache / `rm -rf` the entry |
| `ImageBuildFromExist` | `systemctl freeze`, tar the overlay `upper`, `oras push` it under the new ref, `systemctl thaw` |
| `NetworkConnect` / `Disconnect` / `List`, `ImageBuild` | `ErrEngineNotImplemented`: process pods use the host network and build elsewhere |

`ImagePush` has nothing left to do — `ImageBuildFromExist` pushes the artifact as it builds it.

Resources land on cgroup v2 unit properties: `AllowedCPUs`, `AllowedMemoryNodes` and `CPUWeight`
for bound CPUs, `CPUQuota` otherwise, then `MemoryMax`, `MemoryHigh`, `TasksMax` and the four
`IO*Max` knobs per device.

Two per-workload options ride in the deploy request's raw args:

| Key | Type | Meaning |
| --- | --- | --- |
| `raw` | bool | Run on the host filesystem: no `RootDirectory=`, and the working directory defaults to the unpacked bundle. Such a workload has no filesystem boundary, so `ImageBuildFromExist` refuses it |
| `tasks_max` | int | `TasksMax=` for the unit |

Node prerequisites: systemd ≥ 244 on cgroup v2, `sshd`, `oras`, and a writable `process.root`.

### The meta file

A bare process carries no labels, so core writes `/run/eru/workloads/<id>.json` in the same
session that creates the workload and deletes it in the one that removes it. The record carries
the workload's identity, labels, healthcheck, published ports, cgroup path and journal unit —
this is what eru-agent reads to discover process workloads, and what the engine reads back to
place an exec scope. The directory is on tmpfs, so the records vanish with the transient units on
reboot.

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
