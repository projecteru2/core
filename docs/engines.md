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
| `cocoon://` | `engine/cocoon` | VMs through the cocoon CLI, over SSH |
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
task's stdio lives in node-local FIFOs, so `Execute` and the copy verbs run `ctr tasks exec` on
the node and `VirtualizationAttach` relays the FIFOs themselves with `cat`; the node's own CPU,
memory and disk totals come from `/proc` and `df`; the block devices the IOPS knobs name are
resolved with `stat`; and the node's architecture (`uname -m`) is what the client matches an
image's manifests against, since core's own platform is not the node's.

| `engine.API` | containerd |
| --- | --- |
| `VirtualizationCreate` | `NewContainer` with a new snapshot and the rendered OCI spec; the container id **is** the workload name, so eru-agent reads appname, entrypoint and ident straight off it, and it is also the workload's hostname — which caps it at 64 bytes, `HOST_NAME_MAX` |
| `VirtualizationStart` | `NewTask` with the log-shim `LogURI`, `task.Start`, then `containerd.io/restart.status=running`; a workload created with `open_stdin` takes node-side FIFOs instead of the log URI, with three SSH sessions already relaying them |
| `VirtualizationStop` | `containerd.io/restart.status=stopped` first so the restart plugin does not race, then the image's stop signal and `SIGKILL` after the grace period, then `task.Delete`. A plain stop takes `containerd.stop_timeout`, a forced one kills at once |
| `VirtualizationRemove` | refuses a running workload unless forced, kills the task and deletes the container with its snapshot |
| `VirtualizationSuspend` / `Resume` | `task.Pause` / `task.Resume` |
| `VirtualizationInspect` | the container record for labels and spec, one task-service `Get` for the running state |
| `VirtualizationWait` | `task.Wait` |
| `VirtualizationLogs` | `journalctl SYSLOG_IDENTIFIER=eru ERU_ID=<id>` over SSH, with `-n`, `--since` and `--until`; a followed stream ends when the task exits |
| `VirtualizationAttach` | without stdin, the journald follow; with stdin, the three sessions relaying the workload's FIFOs |
| `VirtualizationResize` | nothing: the stdio is a pipe, and a pipe has no geometry |
| `Execute` / `ExecResize` / `ExecExitCode` | `ctr tasks exec` over the SSH session: stdio is the session's, the TTY is the session's pty, and the exit status is the session's |
| `VirtualizationUpdateResource` | `container.Update` of the stored spec **and** `task.Update` of the live cgroup, so a restart replays the new limits |
| `VirtualizationCopyTo` / `CopyFrom` | a tar stream through `ctr tasks exec`; a workload whose task has not started yet is written into through its own snapshot, mounted on the node with `ctr snapshots mounts` |
| `ImagePull` / `ImageList` / `ImageRemove` / `ImagesPrune` | `client.Pull` with unpack, the image store, and a prune of every image no container is built on |
| `ImageBuildFromExist` | pause the task, diff the workload's snapshot against its image, then under one lease write the new config and manifest into the content store and tag them |
| `ImageBuild` | BuildKit, see below |
| `NetworkList` | the CNI conf dir (`/etc/cni/net.d`) |
| `NetworkConnect` / `Disconnect` | `ErrEngineNotImplemented`: under CNI a network is attached when the netns is created |

### Exec

`Execute` runs `ctr` on the node, so `ctr` is a prerequisite alongside containerd itself, and it
streams through the SSH session rather than through the containerd API:

```
ctr --address <containerd.socket> --namespace <containerd.namespace> tasks exec --exec-id <id> [--tty] [--user U] [--cwd D] <workload> [env K=V …] <cmd…>
```

`ctr tasks exec` in containerd 2.3.4 takes only `--cwd`, `--tty`, `--detach`, `--exec-id`,
`--fifo-dir`, `--log-uri` and `--user` — there is no env flag — so the deploy's environment rides
as an `env K=V …` prefix inside the container. That makes `/usr/bin/env` an image requirement for
`exec` with an environment, and `tar` one for `copy` into or out of a *running* workload; a
workload with no task is copied into through its snapshot instead and needs neither. `ctr` builds
the exec's process from the container's own spec, so `ExecConfig.Privileged` has nothing to add:
an exec always runs with the capabilities the workload itself was created with. `ExecResize` is
the SSH session's own window change — `ctr` forwards its console geometry to the exec on
`SIGWINCH`, so setting the process's size behind it would only be overwritten again.

A deploy's `--file` arrives before the workload starts, so `VirtualizationCopyTo` has no task to
exec in. For a container with no task the engine mounts the container's own snapshot on the node
— `ctr snapshots mounts` prints the mount command, which the same SSH session evaluates — untars
into it and unmounts. The exec path stays for a running workload.

### Interactive workloads

A task has fifos **or** a log URI, never both, so a workload deployed with `open_stdin` cannot be
given the log shim. `ctr tasks start`, which makes the fifos itself and relays them to its own
stdio, cannot carry the pipe eru attaches with either: on containerd 2.3.4 a non-tty `ctr tasks
start` neither delivers piped stdin to the task nor propagates the pipe's EOF, so the workload
reads nothing and never sees the end of its input.

Core therefore owns the fifos. `VirtualizationStart` mkfifos `stdin`, `stdout` and `stderr` under
`/var/lib/eru/containerd/<id>/fifo/` in one SSH round trip, parks three more SSH sessions on them
— `cat > …/stdin`, `cat …/stdout`, `cat …/stderr` — and only then creates the task with those
three paths and `Terminal=false`. The order is load-bearing: the shim opens the stdout and stderr
fifos write-only and blocks until a reader is there. Those three sessions are what
`VirtualizationAttach` hands back.

The semantics are a pipe's. Closing core's write side ends the writing `cat`, and task exit closes
the shim's ends, which is the readers' EOF. The workload's *own* stdin EOF does not follow from
the relay exiting: the shim holds the stdin fifo open read-write, so its read never ends on its
own. That is containerd's design, and the client is the one that has to say when input is done —
`task.CloseIO(ctx, client.WithStdinCloser)`, exactly as `ctr` and docker do it. Core sends it when
the stdin relay ends *cleanly*; a relay that died carried no input, so its end is a failure to
report rather than an EOF to forward. There is no terminal anywhere and no `SIGWINCH`, so
`VirtualizationResize` has nothing to send — a real tty would need a deploy flag eru does not
have. `VirtualizationWait` is plain `task.Wait` for every workload, interactive or not, because
core owns the task either way.

The relays are the one thing this engine starts that outlives the request that started it, so
they are opened on a `context.WithoutCancel` of the deploy's context. Every other SSH session core
holds — the `journalctl -f` follow and an exec — is consumed by a streaming RPC and *should* die
with that RPC's context, and does. A relay bound the same way is closed the moment the create
phase completes: the remote `cat` lingers, so the fifos still look attached from the node, but
core's end of the channel is gone and the first write to stdin fails with EOF.

Each relay is watched. A relay that ends on its own is only normal when it ends quietly — `cat`
reaching EOF as the task exits — so an exit code, a wait error or anything the node wrote to the
relay's stderr is logged against the workload id, and a relay already dead by the time the task
starts fails `VirtualizationStart`. A stdin relay that dies while still blocked opening its fifo
leaves nothing on the node and hangs the workload forever, which is precisely the failure that
must not be silent.

The relays are closed once that exit is consumed, or when the workload is removed, and the fifo
directory goes with the workload directory. Until then they hold three of the eight SSH sessions a
node allows ([core#670](https://github.com/projecteru2/core/issues/670)). Such a workload has no
log shim and nothing in journald: its output travels the stream, exactly as it did with docker.
Everything else keeps `NewTask` + `LogURI`.

### Resources

Every eru knob lands on the OCI spec: `cpuset.cpus` and `cpuset.mems` for bound CPUs and NUMA,
CPU shares split so that a bound whole core keeps the default
weight, `memory.max` with swap pinned to the same value and a reservation of half the limit
(never under 4 MiB), `pids.max`, the four `io.max` rates per device, rlimits, devices,
capabilities, sysctls and privileged mode. Volumes become bind mounts, with the source expanded
against the workload's environment and created before the container starts.

Two things the daemon used to do for core have no containerd equivalent and are done from the
node side. `/etc/hosts` is always generated under `/var/lib/eru/containerd/<id>/` — the localhost
preamble, `127.0.1.1 <id>` so the workload's own hostname resolves, then any `--add-host` entries
— and bind-mounted. `/etc/resolv.conf` is the node's own unless the deploy names its own DNS, in
which case it is generated beside the hosts file.

### The image config

Core applies what the image declares — env, `Entrypoint`+`Cmd`, `WorkingDir`, `User` and
`StopSignal` — by reading the image's config blob itself. containerd's own
`oci.WithImageConfig` cannot be used: it ends in `WithAdditionalGIDs` (and `WithUser` when the
image names one), which temp-mounts the rootfs *on the client* to read `/etc/group` and
`/etc/passwd`. Over a forwarded socket that mount is a node path opened on core's side, and the
create fails with `no such file or directory` under the snapshotter's directory. Core does the
same lookup, but on the node.

The image's env comes first and the deploy's env is merged over it. Process args follow docker:
the image's `ENTRYPOINT` always leads, a deploy command replaces the image's `CMD` after it, and
with no deploy command the image's `ENTRYPOINT`+`CMD` stand. `StopSignal` is stored as
containerd's `io.containerd.image.config.stop-signal` label and is the signal
`VirtualizationStop` sends.

A deploy's `user` must be numeric (`uid` or `uid:gid`, an unnamed group being root) or `root`,
whose ids the passwd contract fixes; any other name is refused with `ErrInvalidEngineArgs` rather
than silently running as root. A name in the *image* config is resolved instead of refused: the
snapshot the container will run on already exists at that point, so the node mounts it once, reads
`etc/passwd` and `etc/group`, and the spec gets the uid, the gid and the supplementary groups.
An image whose passwd has no such entry fails the create. The workload's own name is its
hostname.

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

containerd keeps no log store, so a task that is not interactive is created with

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

## cocoon

`cocoon://[user@]host[:port]` nodes run VMs. Core reaches them the way it reaches a process node —
over SSH with the key pair in the `ssh` config block, the endpoint's user overriding `ssh.user` —
and drives the node's [cocoon](https://github.com/cocoonstack/cocoon) CLI, `journalctl` and
`oras`. cocoon has no remote API and its daemon is optional: the engine talks to the CLI only.
`cocoon.binary` names the command core runs (a sudo wrapper works, so core's login need not be
root), `cocoon.root` holds the durable copy of each workload record, and `cocoon.run_dir` and
`cocoon.cgroup_parent` mirror cocoon's own `run_dir` and `cgroup_parent`, which is where the engine
finds a guest's console and cgroup scope. Every verb is one SSH session and one round trip; create is
two, and is dominated by cocoon's own work; start and resume add a socket forward for `vm.info` and
a second round trip for the meta rewrite.

The VM's cocoon name is the workload id — a 32-hex id core generates, exactly as for a process
workload — so every later verb runs `cocoon vm <verb> <id>` without a lookup, and eru-agent keys
the cocoon daemon's events on it. The eru name stays in the meta file and in core's store.

| `engine.API` | cocoon |
| --- | --- |
| `VirtualizationCreate` | `vm create --output json --name <id> [--cpu N] [--memory B] [--storage B] [--data-disk …] [--network <name>] [--windows \| --user U] <image>` — no boot; then the meta record is written. A failure after the create removes the VM again |
| `VirtualizationStart` | `vm inspect`, `vm start` and `vm inspect` again in one script; then this boot's console is read from Cloud Hypervisor's `vm.info` over a forward of the VM's `api.sock`, and both copies of the meta record are rewritten with it and with the VMM pid. A `vm.info` core cannot reach keeps the recorded socket path, warns once per boot and starts the guest anyway. A Windows guest on its first boot gets its address programmed through `vm exec` in the background, after the start has already returned |
| `VirtualizationStop` | `vm stop`, `--force` for a forced stop, `--timeout` when a grace period is given; a workload with no record on the node is `ErrWorkloadNotExists`, as for the other verbs. cocoon's stop is idempotent, so stopping a created or already-stopped guest succeeds |
| `VirtualizationRemove` | `vm rm [--force]`, then the hibernate snapshot and both copies of the meta record; a running guest is refused unless forced |
| `VirtualizationSuspend` / `Resume` | `vm hibernate --name eru-<id>` / `vm restore --restore-mode copy` followed by `snapshot rm` and `vm inspect`, then the record rewrite as at start. The restore copies, so the delete is best-effort: a snapshot that will not go leaves garbage on the node rather than aborting a resume whose guest is already running |
| `VirtualizationInspect` | the stored record then `vm inspect`: running when the state is `running`, the image, the CNI address under the network's name, and the deploy's `user` — cocoon's own JSON has no eru user, and returning an empty one made core overwrite the stored value after every start |
| `VirtualizationWait` | `vm status --event --format json` until the guest leaves `running`; a VM has no exit code, so the result is 0 |
| `VirtualizationLogs` | `journalctl SYSLOG_IDENTIFIER=eru ERU_ID=<id>` with `-n`, `--since` and `--until` — eru-agent copies the guest console into journald; a followed stream ends when the guest stops, and both `journalctl` and the status watcher are killed by pid rather than through a pipeline, since the watcher only notices a closed pipe at its next event |
| `VirtualizationAttach` | without stdin, the journald follow; with stdin, `ErrEngineNotImplemented` — the console needs a pty (core#660) |
| `Execute` / `ExecExitCode` | `vm exec [-i] [-e K=V …] <id> -- <cmd>` through cocoon-agent in pipe mode, stdio on the SSH session, the exit code the guest command's. `ExecResize` is `ErrEngineNotImplemented` (core#660). A `user` and a `working_dir` are applied inside a Linux guest by wrapping the command — `runuser -u U -- env --chdir=D <cmd>` for a bare user name, `setpriv --reuid=U --regid=G --clear-groups -- env --chdir=D <cmd>` when the id is numeric or a group is named — so the directory is entered as the target user; on a Windows guest both are `ErrEngineNotImplemented` |
| `VirtualizationCopyTo` / `CopyFrom` | a one-entry tar through `vm exec … tar -x -P -f -` / `tar -c -P -f -`: the absolute entry name makes tar create the parents, and `tar.exe` ships with Windows 10+. A copy into a guest that is not running is `ErrEngineNotImplemented` — the state is checked first, one round trip per file |
| `VirtualizationUpdateResource` | a remap (the cpumem binding refresh core runs after every deploy) is a no-op without a round trip; a realloc is `ErrEngineNotImplemented`, CPU and memory hot-plug wait on cocoon (core#661) |
| `ImagePull` | `image pull <ref>` for OCI VM images and cloud-image URLs, registry auth left to cocoon's own config; a split-qcow2 artifact (the Windows images) is `oras pull`ed and `image import`ed under the same ref, once |
| `ImageList` / `ImageRemove` | `image list --format json` filtered by name prefix / `image rm`. An empty store answers `No images found.` in prose rather than `[]`, and reads as an empty list, not a failed node |
| `ImageLocalDigests` / `ImageRemoteDigest` | `image inspect` / `oras manifest fetch --descriptor`; a cloud-image URL is its own digest, so it is pulled once. A node without `oras` (probed with `command -v`) reports no remote digest, so every deploy runs `image pull`, which cocoon answers from its cache. Only a node that answered yes is remembered — a probe an ssh failure lost is asked again, instead of pinning the node as oras-less for the engine's life |
| `ImageBuildFromExist` | `ErrEngineNotImplemented`, and so is `ImagePush`: cocoon has no registry push, so a build from an existing workload can never finish. Saving a snapshot first only left node state behind — core's build always goes on to push the refs and then runs `ImageRemove` over them — so the engine refuses before anything is written |
| `NetworkList` | the CNI conf dir (`/etc/cni/net.d`); `NetworkConnect` / `Disconnect` are `ErrEngineNotImplemented` |
| `ImageBuild`, `ImagesPrune`, `RawEngine` | `ErrEngineNotImplemented` |

### Resources and networks

`--cpu` is the cpumem quota rounded up, `--memory` its memory limit and `--storage` the storage
plugin's quota, in bytes; a knob eru leaves at zero is left to cocoon's default. Each volume the
storage plugin allocates (`src:dst:mode:size`) becomes a data disk of that size, mounted at `dst`
by cloud-init on a Linux guest and left unformatted on a Windows one; a volume without a size is
refused, since a VM has no bind mounts.

A deploy names at most one network, the CNI conflist cocoon should use; none means cocoon's
default conflist. Two networks are refused with `ErrInvalidEngineArgs`. cocoon's IPAM assigns the
address, so an address in the request — which is what `replace --network-inherit` sends back, the
old guest's `{conflist: ip}` — keeps only its conflist name and is logged at debug: a VM cannot be
given a fixed IP. The address lands in the meta file and in `VirtualizationInspect` under the
network's name, `default` only when cocoon itself names no conflist.

A Linux cloud image gets its login from cloud-init, so a deploy that sets no `user` leaves the
guest on cocoon's default guest password. Set the entrypoint's `user` for anything reachable.

An exec that names a user or a working dir costs one extra `vm inspect`, to tell a Linux guest from
a Windows one before the wrapper is rendered. Core hands every entrypoint hook the workload's user,
so a VM workload deployed with `user` pays that round trip on each hook, and every hook runs under
that user. `runuser`, `setpriv` (both util-linux) and `env` (coreutils ≥ 8.28) must be present in
the guest. A bare user name goes to `runuser`, which takes the primary group and the supplementary
groups from the guest's own passwd — the engine cannot, and `setpriv --regid` rejects a name that is
not a group, which `nobody` is not on Debian. A numeric id, or any `user:group`, goes to `setpriv`
with exactly that pair and no supplementary groups; an unnamed group repeats the uid. The env the
hook carries survives both wrappers, which is why neither is a login shell.

A VM has no way to take a file before it boots: the copy verbs go through cocoon-agent inside the
guest, and core sends a deploy's `--file` set between the create and the start. Such a deploy fails
with `ErrEngineNotImplemented` and "send them after the vm boots" rather than a tar error from a
guest that does not exist yet; the same holds for `replace --copy`. Files reach a VM through
`eru-cli send` once it is running, or through the image.

### Windows guests

The deploy request's raw args carry the OS marker:

| Key | Type | Meaning |
| --- | --- | --- |
| `os` | string | `windows` boots the guest with `--windows` (UEFI, `kvm_hyperv=on`, no cidata) |

Windows has no cloud-init and only takes DHCP, while eru's CNI networks use host-local IPAM, so
on the guest's first boot the engine programs the recorded address, mask and gateway with
`netsh interface ip set address Ethernet static …` through `vm exec`, retrying every two seconds
until cocoon-agent answers, for up to three minutes. That loop does not run on the start path:
`VirtualizationStart` returns once `vm start` and the record refresh are done, and the retries go
on in the background under their own five-minute deadline, logging a warning if the guest never
answers. A first boot slower than `global_timeout` — which the pull, the create and the start all
share — would otherwise roll the deploy back and destroy the guest that was still booting; the
entrypoint's health check is what decides the workload is up. `--user` is not passed for a Windows
guest.
Exec and copy go through the agent as on Linux; console-API programs that bypass stdout are the
documented limitation. Resources cannot change on a running Windows guest: a realloc is a
recreate.

A split-qcow2 artifact never satisfies the digest check: cocoon stores the imported disk as a
cloudimg entry whose id is a content sum, while `ImageRemoteDigest` reports the oras manifest
digest, so the two never match and every deploy runs `ImagePull` again. That is not a re-download —
the import script exits at once when `image inspect <ref>` already answers — but it does mean a
Windows deploy always pays one extra round trip.

### The meta file

The same record the process engine writes, at `<cocoon.root>/<id>.json` and
`/run/eru/workloads/<id>.json`, refreshed on tmpfs at start and deleted at remove. Both copies are
written to a temporary name and renamed into place, so a node that loses power mid-write keeps the
previous record instead of a truncated one. For a VM it
carries `kind: vm`, `user`: the login the deploy asked for, which is also what `VirtualizationInspect`
reads back, `cgroup: /sys/fs/cgroup/<cocoon.cgroup_parent>/vm-<cocoon vm id>.scope`,
`netns_pid`: the VMM's pid (cocoon's `pid`; the tap lives in the VM's netns, so eru-agent reads it
from `/proc/<pid>/net/dev`), `iface`: the first tap cocoon created, and `log: {"console_socket": …}` — the guest console of the
current boot, which eru-agent reads and forwards into journald (`cocoon vm logs` shows only the
VMM's own log). A direct-boot OCI image gets a Cloud Hypervisor pty, `/dev/pts/N`, new on every
boot; a UEFI cloud image gets the serial socket
`<cocoon.run_dir>/<cloudhypervisor|firecracker>/<cocoon vm id>/console.sock`, which is also what
create records before the first boot. The field keeps its name for a pty; start and resume rewrite
it and the pid, and eru-agent re-reads the file when it reconnects.

Cloud Hypervisor creates `api.sock` with mode 0700 and root ownership, so reading the pty of a
direct-boot image needs a login that owns it — root, or the login `cocoon.binary` runs as — until
[cocoonstack/cocoon#201](https://github.com/cocoonstack/cocoon/issues/201) relaxes the mode. Every
other login gets `open failed` from sshd, and the record keeps the serial socket path; the guest
still boots, and only its console logs are missing.

### Node prerequisites

cocoon, with `cocoon daemon` as a systemd service (the engine does not need it, eru-agent uses it
for events), the cocoonstack `dev` builds of Cloud Hypervisor, Firecracker and
rust-hypervisor-firmware (the Windows fixes live there), the CNI plugin binaries in `/opt/cni/bin`
with conf in `/etc/cni/net.d`, cocoon-agent inside the guest images (with `runuser`, `setpriv` and
`env` there too, for execs that name a user or a working dir), `sshd` with core's key in
`authorized_keys`, and a login that may run `cocoon.binary` and owns `cocoon.root` and
`/run/eru/workloads` (create them and `chown` them to that login before the first deploy).

`cocoon.root`, `cocoon.run_dir` and `cocoon.cgroup_parent` in core's config must match the node's
own cocoon config. Core never reads cocoon's config — it derives the meta record's `cgroup` path
and the guest's console path from its own values — so a mismatch leaves eru-agent watching a scope
and a socket that do not exist, and the workload never reports healthy.

`cocoon.run_dir` is created by the node info probe itself, which reads `/etc/machine-id`, `nproc`,
`MemTotal` and `df -Pk <cocoon.run_dir>`; a probe that cannot read any of them fails the `AddNode`,
so a node is never registered with zero CPU, memory and storage.

`oras` is optional: without it the remote digest check is skipped and every deploy runs
`image pull`; with it (and the node's own registry credentials) the check runs and split-qcow2
artifacts can be pulled.

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
`setpriv --reuid --regid --init-groups` for a raw one, and enters a working directory other than
`/` with `env --chdir`, which survives the chroot. `env` resolves *inside* the new root, so only
an exec that names such a directory requires the image to carry it — the default lands where
`chroot` already put it and needs nothing. `ImagePush` has nothing left to do — `ImageBuildFromExist` pushes the
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
coreutils for `chroot`, and a writable `process.root`.

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
