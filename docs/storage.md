# Storage

Core is stateless: everything it knows lives in the store. Two backends implement the same
`store.Store` interface and the same key layout — etcd (`store: etcd`, the default) and redis
(`store: redis`).

## Key layout

etcd keys are namespaced by `etcd.prefix` (default `/eru`); redis uses the same paths as plain
keys. `{}` marks a substituted value.

| Key | Holds |
| --- | --- |
| `/pod/info/{podname}` | Pod metadata |
| `/node/{nodename}` | Node metadata: endpoint, pod, labels, bypass, test |
| `/node/{podname}:pod/{nodename}` | Pod-to-node index; listing a pod's nodes is a prefix scan here |
| `/node/{nodename}:ca`, `:cert`, `:key` | The node's TLS material, as PEM content |
| `/node/{nodename}:workloads/{workloadID}` | Node-to-workload index |
| `/status:node/{nodename}` | Node liveness, written with a lease. Its presence *is* the node being up |
| `/workloads/{workloadID}` | Workload metadata: pod, node, name, labels, image, env, resources |
| `/deploy/{appname}/{entrypoint}/{nodename}/{workloadID}` | Deploy index; counting a prefix gives instances per node |
| `/status/{appname}/{entrypoint}/{nodename}/{workloadID}` | Workload status, written by the agent with a lease |
| `/processing/{appname}/{entrypoint}/{nodename}/{ident}` | In-flight deploy count for one deploy round |
| `/services/{address}` | One live core instance, written with a lease |
| `/selfmon/active` | The node-status-watcher election key, written with a lease |
| `/resource/cpumem/{nodename}` | The built-in cpumem plugin's own bookkeeping |

The deploy count a strategy sees is `/deploy/...` plus `/processing/...`, which is how two
concurrent deploys of the same app avoid stacking on the same node.

## etcd

`store/etcdv3` (Mercury) wraps the v3 client. Everything below it goes through `meta.KV`, a thin
layer over etcd's KV, Lease and Watch APIs that adds create/update semantics, batch transactions,
`BindStatus` (put a status key leased against an entity key, failing if the entity is gone) and
ephemeral keys.

- **Namespacing** — the client is wrapped with `namespace.NewKV/NewWatcher/NewLease` on
  `etcd.prefix`, so no code has to prepend it.
- **TLS** — enabled only when `etcd.ca`, `etcd.cert` and `etcd.key` are all set.
- **Auth** — `etcd.auth.username` / `password`.
- **Watch streams** — `ServiceStatusStream`, `NodeStatusStream` and `WorkloadStatusStream` are
  etcd watches on a prefix. Each opens the watch *before* the initial `Get`, so nothing is missed
  between the snapshot and the stream.

## redis

`store/redis` (Rediaron) implements the same interface with `go-redis`. Batch operations become
transactional pipelines; `Watch`-style streams become **keyspace notifications** on
`__keyspace@{db}__:{pattern}`, so the redis server must be configured to emit them — core cannot
see status changes otherwise. Ephemeral keys are ordinary keys with a TTL, refreshed on a timer.

One behaviour differs from etcd and is marked as such in the code: `BatchUpdate` checks key
existence before writing rather than doing it in one transaction. Tests run against an embedded
miniredis.

Note that even with `store: redis`, the built-in cpumem resource plugin still uses the etcd
config for its own bookkeeping — see [Resource plugins](resource-plugins.md).

## Locks

`store.CreateLock(key, ttl)` returns a `DistributedLock` with `Lock` and `Unlock`, and
`Lock` returns a context that is cancelled if the lock is lost. TTL is `lock_timeout`.

- etcd — `concurrency.Mutex` under `etcd.lock_prefix` (default `__lock__/eru`)
- redis — `redislock` under `redis.lock_prefix` (default `/lock`)

Core takes three kinds of lock, always in sorted order and always released in reverse:

| Key | Taken by |
| --- | --- |
| `plock_{podname}` | anything that reads or changes node resources: deploy, realloc, remove, dissociate, node resource inspection |
| `clock_{workloadID}` | anything that touches one workload |
| `cnode_op_{podname}_{nodename}` | node operations that must not interleave, notably remap |

A deploy holds the *pod* lock for the whole allocation phase, which is what serializes capacity
decisions across instances.

## Embedded etcd

`--embedded-storage` starts a single-member etcd inside the process, storing its data under
`$TMPDIR/eru-core-etcd`, and hands core an in-process client namespaced by `etcd.prefix`. No
listener is published, so nothing outside the process can reach it — including a second core
instance, the node status watcher of another host, or `etcdctl`.

It is meant for development and tests. For anything else, point `etcd.machines` at a real cluster.

## Ephemeral keys

Three things in core are leases that must be refreshed or they vanish, and each vanishing is
meaningful:

| Key | Refreshed by | If it expires |
| --- | --- | --- |
| `/services/{addr}` | this core instance, every `grpc.service_heartbeat_interval` | clients stop routing to this instance |
| `/status:node/{nodename}` | the node's agent, via `SetNodeStatus` | the node is considered down and its workloads are marked down |
| `/selfmon/active` | the elected watcher, every `ha_keepalive_interval` | another instance takes over the watcher role |
