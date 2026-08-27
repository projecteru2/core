# Architecture

What each package owns, and how a deploy request travels through them.

## Layers

```
core.go            flags, config, logging, gRPC server, signal handling
  rpc              CoreRPC server (Vibranium): pb <-> core types, task counting
  cluster/calcium  the Cluster implementation: locking, transactions, orchestration
    strategy       pure functions: how many workloads land on which node
    resource       Manager (cobalt) over resource plugins
    store          metadata, locks, watch streams: etcd or redis
    engine         per-node runtime clients, behind a cache
    wal            journal in the store for crash recovery
    source         SCM checkout for image builds (github / gitlab)
    discovery      pushes the live instance list to subscribers
  selfmon          leader-elected node status watcher
  metrics          Prometheus collectors and statsd
  client           Go client library for other services
```

The dependency direction is one-way: `rpc` knows `cluster`, `cluster` knows `store`, `resource`,
`engine`, `strategy` and `wal`, and none of those know `cluster`.

## Startup

`core.go` does, in order:

1. Load the config file, then set up zerolog and (if configured) Sentry.
2. If `--embedded-storage`, start a single-member in-process etcd under `$TMPDIR/eru-core-etcd`.
3. `calcium.New` — build the store, the SCM client, the service-discovery watcher, the resource
   manager (loading plugins), the goroutine pool and the WAL; compute this instance's identifier
   as the SHA-256 of its store settings (`store`, `etcd.machines`, `etcd.prefix`, `redis.addr`,
   `redis.db`), so instances sharing a store share an identity.
4. `factory.InitEngineCache` — start the engine liveness sweep and the node-status subscriber.
5. `cluster.DisasterRecover` — replay the WAL (see [Operations](operations.md)).
6. Listen on `bind`, register `CoreRPC`, optionally install the auth interceptors.
7. If `profile` is set, serve `/metrics` and net/http/pprof on it.
8. `RegisterService` — write `/services/<outbound addr>` with a lease and keep it alive.
9. Start the node status watcher (`selfmon`), which competes for a cluster-wide lock.

On `SIGINT`/`SIGTERM`/`SIGQUIT` core closes its stop channel, unregisters the service, gracefully
stops the gRPC server, and waits for in-flight streaming tasks before returning.

## cluster/calcium

`cluster.Cluster` is the interface the rpc layer talks to; `calcium` is its only implementation.
The package splits by concern — `create.go`, `realloc.go`, `remove.go`, `dissociate.go`,
`replace.go`, `build.go`, `image.go`, `copy.go`, `send.go`, `sendlarge.go`, `execute.go`,
`lambda.go`, `log.go`, `network.go`, `node.go`, `pod.go`, `status.go`, `capacity.go`,
`raw_engine.go`, `service.go`, `wal.go`, `remap.go` — over a small shared base:

- `lock.go` — `withWorkloadLocked`, `withNodePodLocked`, `withNodeOperationLocked`. Locks are
  taken in sorted order and released in reverse, and lock keys are `clock_<id>` for a workload,
  `plock_<pod>` for a pod and `cnode_op_<pod>_<node>` for a node operation.
- `utils.Txn` — the if/then/rollback shape used everywhere a resource change and a metadata
  change must agree. If `then` fails, `rollback` runs; if `if` fails, it does not.
- `c.pool` — an ants pool sized by `max_concurrency`, so fan-out over nodes and workloads has a
  bounded goroutine count.

Long-running calls return a channel of messages, which the rpc layer forwards on a server stream.

## store

`store.Store` covers pods, nodes, workloads, deploy and processing counters, status streams,
service registration, ephemeral keys and lock creation. Two implementations —
`store/etcdv3` (Mercury) and `store/redis` (Rediaron) — share the same key layout, chosen by the
`store` config key. See [Storage](storage.md).

## engine

`engine.API` is the runtime abstraction: virtualization lifecycle, exec, image, network and log
operations. `engine/factory` picks the implementation from the node endpoint's scheme and caches
the client, keyed by endpoint plus TLS material. Engines are horizontally decoupled — no engine
imports another; what they share lives in its own package, `engine/sshrunner` for the SSH
transport the containerd and process engines both run on, `engine/journal` for the journalctl
arguments they both render. See [Engines](engines.md).

## resource

`resource.Manager` — implemented by `resource/cobalt` — fans every call out to the loaded
plugins and merges their answers. Plugins do pure calculation and own their own resource
bookkeeping; core owns node and workload metadata. See [Resource plugins](resource-plugins.md).

## How a deploy flows

`CreateWorkload(DeployOptions)` is a server stream; one `CreateWorkloadMessage` per workload.

1. **rpc** converts the request, opens a task, and calls `Cluster.CreateWorkload`.
2. **calcium** validates the options, stamps a random `ProcessIdent` on them and returns the
   channel; the rest happens on a pooled goroutine inside one `utils.Txn`.
3. **Allocate** (`if`): lock every candidate node's pod, journal a
   `allocate-workload` WAL entry naming the nodes, then
   - ask the resource manager for each node's deploy capacity,
   - read the current deploy count per node from the store (deployed + in-flight),
   - hand both to `strategy.Deploy`, which returns `node -> count`,
   - call `rmgr.Alloc` per node for the workload resources and engine params,
   - journal a `create-processing` entry and write the processing counter, so a concurrent deploy
     of the same app/entrypoint sees these workloads before they exist.
4. **Deploy** (`then`): per node, pull the image unless `ignore_pull`, then create each workload
   concurrently. For each one: `VirtualizationCreate`, journal `create-workload` with the new ID,
   write the workload metadata (decrementing the processing counter in the same transaction),
   copy in any files, run the after-create hooks, start it, inspect it back, and send the message.
   Each workload has its own inner `utils.Txn` whose rollback removes both metadata and instance.
5. **Remap**: after a node finishes, core recomputes engine params for that node's existing
   workloads and applies them — this is how shared CPU pools converge. Remap is idempotent,
   skips workloads whose params digest has not moved since the last sweep, and stays
   deliberately outside the transaction; a failed realloc drops the node's digests so the
   next sweep reapplies store truth.
6. **Rollback**: if any workload failed, its resources are given back with `rmgr.RollbackAlloc`
   under the node lock.
7. **Cleanup**: the processing counters are deleted and every WAL entry that reached a stable
   state is committed (deleted). What is left in the WAL is exactly what a crash would need
   replayed.

`CalculateCapacity` runs steps 1–3 without allocating: with a real strategy it returns the plan
the deploy would produce, and with the `DUMMY` strategy it returns each node's raw capacity.

## selfmon, discovery, metrics

- **selfmon** elects one instance cluster-wide (an ephemeral key at `/selfmon/active` with
  `ha_keepalive_interval` TTL) and watches the node status stream, marking a node's workloads down
  when its status key expires.
- **discovery/helium** watches `/services/` and pushes the address set to every
  `WatchServiceStatus` subscriber, on change and on a timer.
- **metrics** registers the collectors described by the resource plugins plus `core_deploy` and
  `pod_node_up`, and mirrors every value to statsd when configured.
