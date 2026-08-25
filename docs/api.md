# gRPC API

Core exposes exactly one service, `pb.CoreRPC`, defined in
[`rpc/gen/core.proto`](https://github.com/projecteru2/core/blob/master/rpc/gen/core.proto).
`make grpc` regenerates the bindings.

Streaming is marked below: **⇊** is a server stream (one request, many messages), **⇅** is
bidirectional. Every unary rpc returns a gRPC status whose code identifies the failing call.

If `auth.username` is set, every call must carry one metadata entry whose *key* is the configured
username and whose *value* is the password. The Go client does this for you — see
[Go client](client.md).

## Meta

| RPC | Request | Description |
| --- | --- | --- |
| `Info` | `Empty` | Version, git revision, build time, Go version, OS/arch, and this instance's identifier (the SHA-256 of its store settings: `store`, `etcd.machines`, `etcd.prefix`, `redis.addr`, `redis.db`) |
| `WatchServiceStatus` ⇊ | `Empty` | The live set of core instance addresses, plus the interval within which the next push is expected |

## Pods

| RPC | Request | Description |
| --- | --- | --- |
| `AddPod` | `name`, `desc` | Create a pod |
| `RemovePod` | `name` | Remove a pod. Locks every node in it first |
| `GetPod` | `name` | One pod |
| `ListPods` | `Empty` | All pods |

## Nodes

| RPC | Request | Description |
| --- | --- | --- |
| `AddNode` | `nodename`, `endpoint`, `podname`, `ca`/`cert`/`key`, `labels`, `resources`, `test` | Connect to the endpoint, read engine info, register the node with the resource plugins, then write its metadata |
| `RemoveNode` | `nodename` | Remove a node. Fails if it still hosts workloads |
| `ListPodNodes` ⇊ | `podname`, `all`, `labels`, `timeout_in_second`, `skip_info` | Nodes of a pod. `all` includes nodes that are down; `skip_info` skips the engine round-trip; a `timeout_in_second` of 0 or less falls back to `connection_timeout` |
| `GetNode` | `nodename`, `labels` | One node |
| `GetNodeEngineInfo` | `nodename` | The node's engine type |
| `SetNode` | `nodename`, `endpoint`, `ca`/`cert`/`key`, `labels`, `resources`, `delta`, `workloads_down`, `bypass` | Update a node. `delta` treats `resources` as a delta; `bypass` is a tri-state (`KEEP`/`TRUE`/`FALSE`) that takes the node out of scheduling; `workloads_down` marks its workloads dead |

`resources` on `AddNode`/`SetNode` is `map<string, bytes>` — one JSON document per resource
plugin name. See [Resource plugins](resource-plugins.md).

## Node and pod resources

| RPC | Request | Description |
| --- | --- | --- |
| `GetPodResource` ⇊ | `name` (pod) | Per-node capacity, usage and diffs across the pod |
| `GetNodeResource` | `opts.nodename`, `fix` | One node's capacity, usage and diffs. Also inspects each workload on the engine; `fix` asks the plugins to rewrite usage from the workloads |

## Status

| RPC | Request | Description |
| --- | --- | --- |
| `GetNodeStatus` | `nodename` | Whether the node's status key is currently alive |
| `SetNodeStatus` | `nodename`, `ttl` | Refresh the node's liveness key for `ttl` seconds. A negative `ttl` deletes it. This is what `eru-agent` calls |
| `NodeStatusStream` ⇊ | `Empty` | Node liveness changes as they happen |
| `GetWorkloadsStatus` | `IDs` | Last reported status of each workload |
| `SetWorkloadsStatus` | `status[]` (with per-entry `ttl`) | Report workload status; also what the agent calls |
| `WorkloadStatusStream` ⇊ | `appname`, `entrypoint`, `nodename`, `labels` | Status changes for the matching workloads, including deletions |

## Capacity

| RPC | Request | Description |
| --- | --- | --- |
| `CalculateCapacity` | `DeployOptions` | How many workloads would fit, and where — without allocating anything. With `deploy_strategy: DUMMY` it returns raw per-node capacity instead of a strategy plan |

## Workload queries

| RPC | Request | Description |
| --- | --- | --- |
| `GetWorkload` | `id` | One workload |
| `GetWorkloads` | `IDs` | Several workloads |
| `ListWorkloads` ⇊ | `appname`, `entrypoint`, `nodename`, `labels`, `limit` | Filtered listing |
| `ListNodeWorkloads` | `nodename`, `labels` | Everything on one node |

## Workload lifecycle

| RPC | Request | Description |
| --- | --- | --- |
| `CreateWorkload` ⇊ | `DeployOptions` | Allocate and deploy. One message per workload, each carrying either the new ID, name and published ports, or an error |
| `ReplaceWorkload` ⇊ | `deployOpt`, `IDs`, `networkinherit`, `filter_labels`, `copy` | Rolling replace: for each old workload, stop it and create a replacement with the same resources. `copy` maps a path in the old workload to a path in the new one, preserving uid, gid and mode; empty `IDs` means every workload of that app and entrypoint |
| `RemoveWorkload` ⇊ | `IDs`, `force` | Stop, remove, and return the resources to the node |
| `DissociateWorkload` ⇊ | `IDs` | Return the resources and drop the metadata, leaving the instance running on the node |
| `ControlWorkload` ⇊ | `IDs`, `type`, `force` | `start`, `stop`, `restart`, `suspend` or `resume`; runs the entrypoint hooks unless `force` |
| `ReallocResource` | `id`, `resources` | Change a running workload's resources in place |
| `ExecuteWorkload` ⇅ | `workload_id`, `commands`, `envs`, `workdir`, `open_stdin` | Exec inside a workload. When `open_stdin` is set, further client messages carry stdin in `repl_cmd` |
| `RunAndWait` ⇅ | `deploy_options`, `cmd`, `async`, `async_timeout` | Lambda: deploy, attach, wait for exit, then remove. The first messages carry the new workload IDs (`TYPEWORKLOADID`), the last output line is `[exitcode] <n>`. With `async`, core sends the IDs, detaches from the stream, forces `open_stdin` off, and logs the output itself under `async_timeout` seconds (default `global_timeout`) |
| `LogStream` ⇊ | `id`, `tail`, `since`, `until`, `follow` | Engine logs for one workload |
| `RawEngine` | `id`, `op`, `params`, `ignore_lock` | Pass an engine-specific operation through to the node's engine. Implemented by the virt engine; the docker and process engines return `ErrEngineNotImplemented` |

## Files

| RPC | Request | Description |
| --- | --- | --- |
| `Copy` ⇊ | `targets` (workload ID → paths) | Copy files out of workloads. Each path is streamed as a tar archive in 4 KiB `data` chunks |
| `Send` ⇊ | `IDs`, `data`, `modes`, `owners` | Push in-memory files into workloads. Files with no uid/gid/mode get `0755` |
| `SendLargeFile` ⇅ | stream of `ids`, `dst`, `size`, `mode`, `owner`, `chunk` | Same, chunked, for files too large for one message |

## Images

| RPC | Request | Description |
| --- | --- | --- |
| `BuildImage` ⇊ | `name`, `user`, `uid`, `tags`, `builds`, `tar`, `build_method`, `exist_id`, `platform`, `node_filter` | Build on the most idle node matching `build.node_filter`, then push. `node_filter` may only narrow that selection. `build_method` is `SCM` (clone via the configured SCM), `RAW` (the `tar` field) or `EXIST` (commit a running workload). Requires `git.scm_type` for `SCM` |
| `CacheImage` ⇊ | `podname`, `nodenames`, `images` | Pull the images on every matching node |
| `RemoveImage` ⇊ | `podname`, `nodenames`, `images`, `prune` | Remove them; with `prune`, also prune dangling images on each node afterwards |
| `ListImage` ⇊ | `podname`, `nodenames`, `filter` | List images per node |

Image references are built as `hub/namespace/appname:tag` from `docker.hub` and
`docker.namespace`; with no tags, `latest` is used.

## Networks

| RPC | Request | Description |
| --- | --- | --- |
| `ListNetworks` | `podname`, `driver` | Networks visible from the first node of the pod |
| `ConnectNetwork` | `network`, `target`, `ipv4`, `ipv6` | Attach a workload to a network; returns its subnets |
| `DisconnectNetwork` | `network`, `target`, `force` | Detach it |

The docker and virt engines implement these; `process://` nodes use the host network and return
`ErrEngineNotImplemented`.

## DeployOptions

The message behind `CreateWorkload`, `CalculateCapacity`, `RunAndWait` and `ReplaceWorkload`.

| Field | Meaning |
| --- | --- |
| `name` | Application name. Required |
| `entrypoint` | Name, `commands`, `dir`, `privileged`, `restart`, `publish`, `sysctls`, `healthcheck`, `hook`, `log`. Required |
| `podname` | Pod to deploy into. Required |
| `image` | Image reference. Required |
| `count` | How many workloads. Must be > 0 |
| `deploy_strategy` | `AUTO`, `FILL`, `EACH`, `GLOBAL`, `DRAINED` or `DUMMY` — see [Deploy strategies](deploy-strategies.md) |
| `node_filter` | `includes`, `excludes`, `labels`, `all` — which nodes are candidates |
| `nodes_limit` | Cap on how many nodes take part; meaning depends on the strategy |
| `resources` | `map<string, bytes>`: one JSON request per resource plugin |
| `env`, `dns`, `extra_hosts`, `networks`, `user`, `labels`, `nodelabels` | Passed to the engine |
| `data`, `modes`, `owners` | Files to place inside each workload at create time |
| `after_create` | Commands to run once the workload exists |
| `open_stdin`, `debug`, `ignore_hook`, `ignore_pull` | Behaviour switches |
| `raw_args` | Engine-specific JSON blob merged into the create request |

Core adds `APP_NAME`, `ERU_POD`, `ERU_NODE_NAME` and `ERU_WORKLOAD_SEQ` to every workload's
environment, and labels each one with `ERU=1`, `ERU_META`, `eru.nodename` and `eru.coreid`.
