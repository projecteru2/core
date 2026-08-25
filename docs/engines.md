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
| `containerd://` | `engine/containerd` | containerd's own API, over an SSH forward of the node's socket |
| `virt-grpc://` | `engine/virt` | yavirt (archived), over the libyavirt gRPC client |
| `process://` | `engine/process` | Bare processes as systemd transient units, over SSH |
| `mock://` | `engine/mocks/fakeengine` | Fully mocked engine, for tests and dry runs |

An endpoint with any other prefix is rejected with `ErrInvaildNodeEndpoint`.

## containerd

`containerd://[user@]host[:port]` nodes run OCI containers. containerd serves only its CRI plugin
on TCP — `plugins/server/grpc` registers on the TCP listener just the plugins implementing
`RegisterTCP`, and CRI is the only one — so the native API (containers, tasks, images, content,
diff, events) is reachable on the unix socket alone. Core therefore dials it the way it dials a
process node: one SSH connection per node, with containerd's gRPC client running over an OpenSSH
socket forward (`direct-streamlocal`, on by default in sshd) of `containerd.socket`. The same
connection carries `journalctl`, `ctr` and the CNI conf listing. `types.Node.{ca,cert,key}` are
unused by this engine.

Everything containerd's API cannot answer from core is asked of the node over that connection: a
task's stdio lives in node-local FIFOs, so `Execute`, `VirtualizationAttach` and the copy verbs
run `ctr` on the node — `tasks exec` for the first two uses, `tasks attach` for the third; the
node's own CPU, memory and disk totals come from `/proc` and `df`; the block devices the IOPS
knobs name are resolved with `stat`; and the node's architecture (`uname -m`) is what the client
matches an image's manifests against, since core's own platform is not the node's.

| `engine.API` | containerd |
| --- | --- |
| `VirtualizationCreate` | `NewContainer` with a new snapshot and the rendered OCI spec; the container id **is** the workload name, so eru-agent reads appname, entrypoint and ident straight off it |
| `VirtualizationStart` | `NewTask` with the log-shim `LogURI`, `task.Start`, then `containerd.io/restart.status=running` |
| `VirtualizationStop` | `containerd.io/restart.status=stopped` first so the restart plugin does not race, then `SIGTERM`, `SIGKILL` after the grace period, then `task.Delete` |
| `VirtualizationRemove` | refuses a running workload unless forced, kills the task and deletes the container with its snapshot |
| `VirtualizationSuspend` / `Resume` | `task.Pause` / `task.Resume` |
| `VirtualizationInspect` | the container record for labels and spec, one task-service `Get` for the running state |
| `VirtualizationWait` | `task.Wait` |
| `VirtualizationLogs` | `journalctl SYSLOG_IDENTIFIER=eru ERU_ID=<id>` over SSH, with `-n`, `--since` and `--until`; a followed stream ends when the task exits |
| `VirtualizationAttach` | without stdin, the journald follow; with stdin, `ctr tasks attach` over the SSH session |
| `VirtualizationResize` | the attach session's window change |
| `Execute` / `ExecResize` / `ExecExitCode` | `ctr tasks exec` over the SSH session: stdio is the session's, the TTY is the session's pty, and the exit status is the session's |
| `VirtualizationUpdateResource` | `container.Update` of the stored spec **and** `task.Update` of the live cgroup, so a restart replays the new limits |
| `VirtualizationCopyTo` / `CopyFrom` | a tar stream through `ctr tasks exec` |
| `ImagePull` / `ImageList` / `ImageRemove` / `ImagesPrune` | `client.Pull` with unpack, the image store, and a prune of every image no container is built on |
| `ImageBuildFromExist` | pause the task, diff the workload's snapshot against its image, write the new config and manifest into the content store and tag them |
| `ImageBuild` | BuildKit, see below |
| `NetworkList` | the CNI conf dir (`/etc/cni/net.d`) |
| `NetworkConnect` / `Disconnect` | `ErrEngineNotImplemented`: under CNI a network is attached when the netns is created |

### Exec and attach

Both run `ctr` on the node, so `ctr` is a prerequisite alongside containerd itself, and both
stream through the SSH session rather than through the containerd API:

```
ctr --address <containerd.socket> --namespace <containerd.namespace> tasks exec   --exec-id <id> [--tty] [--user U] [--cwd D] <workload> [env K=V …] <cmd…>
ctr --address <containerd.socket> --namespace <containerd.namespace> tasks attach <workload>
```

`ctr tasks exec` has no env flag, so the deploy's environment rides as an `env K=V …` prefix
inside the container. `ctr tasks attach` takes no flags at all: it reads `process.terminal` off
the container's own spec and allocates a console only when the spec has one, so core asks the SSH
session for a pty exactly when the workload was created with `open_stdin`. `ExecResize` and
`VirtualizationResize` are both the SSH session's window change — `ctr` forwards its console
geometry to the task on `SIGWINCH`, so setting the task's size behind it would only be
overwritten again.

`ctr tasks attach` deletes the task when the attach ends. Core therefore registers the task's
exit watch *before* starting `ctr`, and `VirtualizationWait` takes the exit status from that
watch: by the time a lambda finishes draining the stream, the task record is already gone.

### Resources

Every eru knob lands on the OCI spec: `cpuset.cpus` and `cpuset.mems` for bound CPUs and NUMA,
CPU shares split so that a bound whole core keeps the default
weight, `memory.max` with swap pinned to the same value and a reservation of half the limit
(never under 4 MiB), `pids.max`, the four `io.max` rates per device, rlimits, devices,
capabilities, sysctls and privileged mode. Volumes become bind mounts, with the source expanded
against the workload's environment and created before the container starts.

Two things the daemon used to do for core have no containerd equivalent and are done from the
node side: `/etc/resolv.conf` and `/etc/hosts` are bind-mounted from the node unless the deploy
asks for its own DNS or extra hosts, in which case core writes them under
`/var/lib/eru/containerd/<id>/` and binds those instead.

### The image config

Core applies what the image declares — env, `Entrypoint`+`Cmd`, `WorkingDir`, `User` and
`StopSignal` — by reading the image's config blob itself. containerd's own
`oci.WithImageConfig` cannot be used: it ends in `WithAdditionalGIDs` (and `WithUser` when the
image names one), which temp-mounts the rootfs *on the client* to read `/etc/group` and
`/etc/passwd`. Over a forwarded socket that mount is a node path opened on core's side, and the
create fails with `no such file or directory` under the snapshotter's directory.

The image's env comes first and the deploy's env is merged over it; the image's entrypoint and
command become the process args only when the deploy names no command — a deploy command
replaces both, it is not appended to the entrypoint. `StopSignal` is stored as containerd's
`io.containerd.image.config.stop-signal` label and is the signal `VirtualizationStop` sends.

`user` must be numeric (`uid` or `uid:gid`, an unnamed group being root): resolving a name needs
the image's `/etc/passwd`, which only the node can read. A named user on the deploy is refused
with `ErrInvalidEngineArgs` rather than silently running as root; a named user in the *image*
config is logged as a warning and the spec's default stands.

### Networking

Core has no netns on the node, so CNI runs there. The spec carries a `createRuntime` and a
`poststop` hook with identical argv:

```
/usr/local/bin/eru-agent oci-hook --network <conflist name> [--socket <containerd socket>]
```

runc invokes `createRuntime` with the runtime state in status `creating` and `poststop` with
status `stopped` and pid 0, which is how the hook tells an attach from a detach. The containerd
namespace reaches the hook as the OCI annotation `eru.namespace`, because a hook is handed the
runtime state and never the container's labels. The hook records each address it gets back as the
container label `eru.network.<name>=<ipv4>`, and that is where `VirtualizationInspect` reads a
workload's networks from.

A host-network workload is one whose spec has **no** `network` entry in `linux.namespaces` — the
nerdctl and CRI convention — and carries no hooks; it inspects as `{"host": <node ip>}`.

### Logs

containerd keeps no log store, so the task is created with

```
cio.LogURI = binary:///usr/local/bin/eru-agent?log-shim
```

containerd execs that binary per task with stdout and stderr on fds 3 and 4, `CONTAINER_ID` and
`CONTAINER_NAMESPACE` in the environment, and the URI's query flattened into argv pairs — a query
key with no value becomes the key followed by an empty positional, which is what makes `log-shim`
arrive as the agent's subcommand. The shim writes each line to journald as
`SYSLOG_IDENTIFIER=eru`, `ERU_ID=<id>`; `journalctl` over SSH reads them back. The same URI is
stored as `containerd.io/restart.loguri` so a restarted task keeps logging.

### Restart policy

`RestartPolicy` maps onto containerd's restart plugin: the policy goes into
`containerd.io/restart.policy` at create, and core drives `containerd.io/restart.status` — the
plugin's *desired* state — at start and stop. A workload with no policy, or `no`, carries neither
label and the plugin never looks at it.

### Builds on BuildKit

The build spec is rendered into one multi-stage Dockerfile exactly as before — `BuildContent`
clones the repo, downloads the artifacts and writes `Dockerfile` next to them, then tars the
directory. `ImageBuild` unpacks that tar into a temporary directory, dials the build node's
`buildkitd` over the same SSH forward (a `tcp://` value in `containerd.buildkit` is dialed
directly instead) and solves it with the `dockerfile.v0` frontend, exporting to the image
exporter with `push=true` and the refs `BuildRefs` composed. Registry credentials reach the
solve as a session auth provider fed from `registry.auths`.

Two consequences: BuildKit pushes the image itself, so the image never lands in the node's own
image store and `ImagePush` has nothing left to send; and `ImageBuildCachePrune` is BuildKit's
`Prune`, whose reclaimed bytes core reports. The build status stream — vertices and their logs —
is converted into the same `BuildImageMessage` stream clients already read.

Which nodes may build is decided by `build.node_filter`, not by the engine.

### Node prerequisites

containerd ≥ 2.0 with the restart plugin, `runc`, `ctr`, the CNI plugin binaries in
`/opt/cni/bin` with conf in `/etc/cni/net.d`, the eru-agent binary at `/usr/local/bin/eru-agent`,
`sshd` with core's key in `authorized_keys`, journald rate limits raised for the `eru` identifier,
and `buildkitd` on the nodes `build.node_filter` selects.

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
never interpolated into a shell line. Sessions are bounded at eight per node, so a wide deploy
queues instead of exhausting sshd's `MaxSessions`.

One transient service per workload (`eru-<id>.service`), one slice per pod (`eru-<pod>.slice`):

```
systemd-run --unit=eru-<id> --slice=eru-<pod>.slice \
  -p Description="<app>/<entrypoint>" \
  -p RemainAfterExit=yes -p SyslogIdentifier=eru \
  -p User=<user> -p WorkingDirectory=<dir> -p RootDirectory=<overlay merged> \
  -p Environment=… -p BindPaths=… \
  -p AllowedCPUs=… -p CPUQuota=… -p MemoryMax=… -p MemoryLow=… -p MemorySwapMax=0 \
  -p Restart=<policy> -p TimeoutStopSec=<process.stop_timeout> \
  -- <cmd> <args…>
```

`RemainAfterExit=yes` is what makes an exited workload still answerable: systemd garbage-collects
a transient unit the moment it goes inactive, and without it `systemctl show` reports `not-found`
and the exit status is gone. `SyslogIdentifier=eru` is the identifier eru-agent's journal reader
matches. A workload is running when its `SubState` is `running`; it exists when its directory
under `process.root` does, which is why a created-but-never-started or an exited workload inspects
as stopped rather than missing.

| `engine.API` | Node command |
| --- | --- |
| `VirtualizationCreate` | copy the artifact from the cache into `<process.root>/<id>/lower`, or `oras pull` it there when the cache has no entry; prepare `upper`, `work` and `merged`, create the bind sources, write the meta record, and render the `systemd-run` command into `<process.root>/<id>/run.sh`. Nothing runs yet, and a failure rolls the directory back |
| `VirtualizationStart` | no-op when the unit is already running; otherwise release the finished unit name, mount the overlay at `merged`, copy the meta record onto tmpfs and run `run.sh` |
| `VirtualizationStop` | `systemctl stop`, then a lazy unmount; a forced stop sends `SIGKILL` to the whole unit first |
| `VirtualizationRemove` | refuses a running workload unless forced, then `systemctl reset-failed`, lazy unmount, and delete the workload directory and the meta record |
| `VirtualizationSuspend` / `Resume` | `systemctl freeze` / `thaw` |
| `VirtualizationInspect` | the workload directory for existence, `systemctl show` for state |
| `VirtualizationWait` | poll `systemctl show -p SubState` until the unit has exited, failed or died, then return `ExecMainStatus` |
| `VirtualizationLogs` | `journalctl -u eru-<id> -o cat`, with `-n`, `--since` and `--until`; a followed stream ends when the unit leaves `running` |
| `VirtualizationAttach` | logs-follow; stdin returns `ErrEngineNotImplemented` |
| `Execute` | `systemd-run --scope` in the workload's slice, entering the root with `chroot --userspec` or dropping privileges with `setpriv`, stdio streamed over the SSH session, exit code from the scope |
| `VirtualizationUpdateResource` | `systemctl set-property --runtime` with the complete knob set — live, no restart |
| `VirtualizationCopyTo` / `CopyFrom` | sftp through the mounted overlay at `merged`; when it is not mounted, writes land in `upper` and reads fall back from `upper` to `lower` |
| `ImagePull` / `ImageList` / `ImageRemove` | `oras pull` into a cleared artifact cache entry / list the cache / `rm -rf` the entry |
| `ImageBuildFromExist` | `systemctl freeze`, tar the mounted overlay at `merged` so the layer is a complete bundle, `oras push` it under the new ref, `systemctl thaw` |
| `NetworkConnect` / `Disconnect` / `List`, `ImageBuild` | `ErrEngineNotImplemented`: process pods use the host network and build elsewhere |

A scope unit has no exec context, so `Execute` cannot pass `RootDirectory=` or `--uid` as unit
properties: it execs `chroot --userspec=<user> <merged>` for an overlay workload,
`setpriv --reuid --regid --init-groups` for a raw one, and enters the working directory with
`env --chdir`, which survives the chroot. `ImagePush` has nothing left to do — `ImageBuildFromExist` pushes the
artifact as it builds it.

Resources land on cgroup v2 unit properties: `AllowedCPUs`, `AllowedMemoryNodes` and `CPUWeight`
for bound CPUs, `CPUQuota` whenever a quota is set, then `MemoryMax`, `MemoryLow` (half the
reservation), `MemorySwapMax=0`, `TasksMax` and the four `IO*Max` knobs per device. Volume
bindings become `BindPaths=` — `BindReadOnlyPaths=` for an `ro` mode — with the source expanded
against the workload's environment and created before the unit starts; a bind needs no
`RootDirectory=`, so raw workloads get them too. `VirtualizationUpdateResource` sends every cgroup
knob it can set, including the empty values that reset one, because `set-property` only touches
what it is given and a realloc has to clear the shape it replaces; mounts are not live-settable
and stay out of it.

`ExecStart` must be absolute, so a relative command resolves against the unit's root, or against
the unpacked bundle for a raw workload.

Two per-workload options ride in the deploy request's raw args:

| Key | Type | Meaning |
| --- | --- | --- |
| `raw` | bool | Run on the host filesystem: no `RootDirectory=`, and the working directory defaults to the unpacked bundle. Such a workload has no filesystem boundary, so `ImageBuildFromExist` refuses it |
| `tasks_max` | int | `TasksMax=` for the unit |

Node prerequisites: systemd ≥ 244 on cgroup v2, `sshd`, `oras`, `util-linux` for `setpriv`,
coreutils ≥ 8.28 for `chroot` and `env --chdir`, and a writable `process.root`.

### The bundle format

A process workload's image is an OCI artifact whose layer carries media type
`application/vnd.eru.process.bundle.v1+tar` and *is a tar of the rootfs*. `oras` stores such a
layer as a file rather than expanding it, so the engine untars every `*.tar` at the top of a
pulled directory in place and removes the archive; an artifact that was pushed as a directory
arrives already expanded and is left alone. `ImageBuildFromExist` writes the same shape back: it
tars the mounted overlay — the complete rootfs, not the `upper` diff — and pushes it under the new
tag with that artifact and layer media type.

### The meta file

A bare process carries no labels, so core writes the workload's record twice: to
`<process.root>/<id>/meta.json`, which survives a reboot, and to `/run/eru/workloads/<id>.json`,
which eru-agent watches. Start refreshes the tmpfs copy from the durable one, and remove deletes
both. The record carries the workload's identity, labels, healthcheck, published ports, cgroup
path and journal unit — this is what eru-agent reads to discover process workloads, and what the
engine reads back to place an exec scope or resolve a copy target.

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
