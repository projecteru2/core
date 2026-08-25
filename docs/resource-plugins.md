# Resource plugins

Core does not know what a "CPU" or a "GPU" is. It knows workloads, nodes and counts; every
statement about *how much* of something a node has, and whether a request fits, comes from a
resource plugin. `resource.Manager` — implemented by `resource/cobalt` — is the fan-out layer
between the cluster logic and the plugins.

## The wire shape

Anywhere the API takes resources it takes `map<string, bytes>`: one JSON document per plugin
name. A deploy asking for CPU and GPU sends

```json
{
  "cpumem": {"cpu-request": 1.2, "cpu-limit": 2.0, "memory-request": "512MB"},
  "gpu":    {"gpu-request": 1}
}
```

and each plugin only ever sees its own value. The same shape is used by `AddNode` and `SetNode`
(`NodeResourceRequest`), stored per workload, and returned in `Node.resource_capacity` /
`resource_usage`.

## How cobalt combines plugins

Every manager method calls all plugins concurrently and merges:

- **Deploy capacity** — a node survives only if *every* plugin returns it. Its capacity is the
  **minimum** across plugins; `usage` and `rate` are weighted averages using each plugin's
  `weight`. That is why "no node meets all the resource requirements at the same time" is a
  distinct error from "not enough capacity".
- **Alloc** — `CalculateDeploy` on each plugin produces per-workload resources and engine params;
  the engine params of all plugins are merged into one blob per workload, then node usage is
  incremented. Prepare/commit/rollback, so a failure gives the resources back.
- **Node info** — capacity, usage and diffs are collected per plugin name. `resource_plugin.whitelist`,
  if set, restricts *this* call to the listed plugins.
- **Most idle node** — every plugin nominates a node with a priority; the highest priority wins.
  This is how a build node is chosen.

Errors from different plugins are combined, not swallowed — one failing plugin fails the call.

## The plugin interface

Whatever the transport, a plugin answers the same set of questions
(`resource/plugins/plugin.go`):

| Method | Purpose |
| --- | --- |
| `Name` | The key this plugin owns in every resource map |
| `CalculateDeploy` | Pure calculation: for `n` workloads on a node, the resources and engine params of each |
| `CalculateRealloc` | New engine params, delta and final resources for an existing workload |
| `CalculateRemap` | Engine params for every workload on a node, after something changed |
| `AddNode` / `RemoveNode` | Create or drop the plugin's bookkeeping for a node |
| `GetNodesDeployCapacity` | How many workloads of this shape fit on each node, plus usage and rate |
| `SetNodeResourceCapacity` | Change total capacity (absolute or delta, increment or decrement) |
| `SetNodeResourceUsage` | Change allocated usage the same way |
| `GetNodeResourceInfo` | Capacity, usage, and diffs between the node's bookkeeping and its workloads |
| `SetNodeResourceInfo` | Overwrite both, absolute — used to roll back `RemoveNode` |
| `FixNodeResource` | Rewrite usage from the workloads it is given |
| `GetMostIdleNode` | Nominate a node, with a priority |
| `GetMetricsDescription` / `GetMetrics` | Declare Prometheus collectors, then produce values |

The calculate methods must be side-effect free: core calls them inside transactions and may
discard the result.

## cpumem — the built-in plugin

Always loaded, under the name `cpumem`. It stores its own bookkeeping in etcd at
`/resource/cpumem/<nodename>` (under `etcd.prefix`), reusing core's etcd config — including the
embedded cluster when `--embedded-storage` is on. With `store: redis` and no etcd endpoints
configured, plugin construction fails, so a redis deployment still needs `etcd.machines`.

It reads two scheduler settings:

- `scheduler.sharebase` — pieces per core, i.e. the granularity of fractional CPU requests
- `scheduler.maxshare` — how many cores may be left fragmented; `-1` for no limit

On `AddNode`, when the request does not spell out a CPU map, cpumem builds one from the engine's
reported CPU count at `sharebase` pieces each, and takes memory as **80%** of the engine's
reported total. If the engine's info carries a `cpumem` blob of its own, that
blob wins over the engine's generic numbers, and it also supplies the NUMA topology.

## Binary plugins

Any **executable file directly inside `resource_plugin.dir`** (subdirectories are not scanned) is
loaded as a binary plugin. Its file name becomes its plugin name.

The contract is a subcommand plus JSON:

```
<plugin> <subcommand>  < request.json  > response.json
```

- The request is one JSON object on stdin.
- The response is one JSON object on stdout. Core reads combined output, so anything written to
  stderr becomes part of what it tries to parse — keep it quiet on success.
- Empty output is accepted and leaves the response zero-valued.
- The working directory is `resource_plugin.dir`.
- Each invocation is killed after `resource_plugin.call_timeout`.

| Subcommand | Method |
| --- | --- |
| `calculate-deploy` | `CalculateDeploy` |
| `calculate-realloc` | `CalculateRealloc` |
| `calculate-remap` | `CalculateRemap` |
| `add-node` | `AddNode` |
| `remove-node` | `RemoveNode` |
| `get-nodes-deploy-capacity` | `GetNodesDeployCapacity` |
| `set-node-resource-capacity` | `SetNodeResourceCapacity` |
| `get-node-resource-info` | `GetNodeResourceInfo` |
| `set-node-resource-info` | `SetNodeResourceInfo` |
| `set-node-resource-usage` | `SetNodeResourceUsage` |
| `get-most-idle-node` | `GetMostIdleNode` |
| `fix-node-resource` | `FixNodeResource` |
| `get-metrics-description` | `GetMetricsDescription` |
| `get-metrics` | `GetMetrics` |

Response shapes are the JSON encodings of the types in `resource/plugins/types/`, e.g.
`calculate-deploy` returns `{"engines_params": [...], "workloads_resource": [...]}` and
`get-nodes-deploy-capacity` returns `{"nodes_deploy_capacity_map": {...}, "total": n}`.

[resource-extend](https://github.com/projecteru2/resource-extend) implements gpu and storage
plugins this way.

## Load order and name collisions

At startup cobalt loads `cpumem`, then every executable in `resource_plugin.dir`. A plugin whose
`Name()` is already taken is skipped, so `cpumem` cannot be shadowed.
