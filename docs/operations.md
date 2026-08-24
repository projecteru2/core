# Operations

Running core: what recovers after a crash, what keeps node state honest, and what to watch.

## WAL and disaster recovery

Core journals to a local bbolt file at `wal_file`, opened with `wal_open_timeout`. The journal is
*intent*, not state: an entry is written before the risky step and deleted once the step has
reached a stable outcome. So whatever survives in the file after a crash is exactly the work that
was in flight, and `DisasterRecover` replays it at startup, before the gRPC server starts serving.

| Event | Written before | Replay does |
| --- | --- | --- |
| `allocate-workload` | resources are allocated on a set of nodes | re-derives each node's usage from its actual workloads (`NodeResource` with `fix`) |
| `create-workload` | a workload's ID is known but its metadata is not yet stored | removes the workload — from the store if it is there, otherwise straight off the engine |
| `create-processing` | an in-flight deploy counter is written | deletes the stale counter, so it stops inflating deploy counts forever |
| `create-lambda` | a `RunAndWait` workload starts | waits for it to exit, then removes it |

Each replayed handler gets a 32-second deadline. Entries whose handler is unknown are logged and
skipped; entries that fail are logged and left in place for the next start.

Because the file is local, **the WAL belongs to one instance**. Two core instances must not share
a `wal_file` path, and a replacement instance on a different host will not clean up its
predecessor's in-flight work — bring the old host's core back, or reconcile with
`GetNodeResource(fix: true)`.

The two `NodeResource`/`PodResource` calls are the manual counterpart: they list a node's
workloads, ask the plugins for capacity and usage, inspect each workload on the engine, and report
`diffs`. With `fix: true`, the plugins rewrite usage from the workloads that actually exist.

## Node status watcher (selfmon)

Every core instance starts a node status watcher, but only one is active at a time. They compete
for an ephemeral key at `/selfmon/active` with a `ha_keepalive_interval` TTL; the winner runs, the
losers retry every second and log once a minute. If the winner dies, its key expires and another
takes over.

The active watcher:

1. Lists every node in every pod and reads each node's status once, so it starts from a known
   state rather than waiting for the next change.
2. Subscribes to the store's node status stream.
3. When a node goes *down* — its `/status:node/{nodename}` key expired, meaning its agent stopped
   reporting — calls `SetNode` with `workloads_down`, marking that node's workloads dead.

It deliberately ignores nodes coming *back* up: that transition is owned by the agent, which
re-reports the node and its workloads. Nodes registered with `test: true` are always treated as
alive.

Separately, the engine cache subscribes to the same stream and evicts every cached engine client
belonging to a node that went down — see [Engines](engines.md).

## Metrics

Set `profile` to a `host:port` to expose an HTTP server with `/metrics` in Prometheus format and
the standard net/http/pprof handlers. With `profile` empty, neither is served.

Scraping `/metrics` is not passive: the handler first walks every node, sends its up/down gauge
and asks the resource plugins for that node's metrics, then serves the registry. A scrape
therefore costs one pass over the cluster and is bounded by `global_timeout` — keep the scrape
interval well above it.

Two metrics are core's own:

| Metric | Type | Labels | Meaning |
| --- | --- | --- | --- |
| `core_deploy` | counter | `hostname` | Workloads this instance has scheduled |
| `pod_node_up` | gauge | `hostname`, `podname`, `nodename` | 1 when the node is neither down nor bypassed |

Everything else is declared by the resource plugins through `GetMetricsDescription`, so a cluster
running the cpumem plugin plus gpu and storage plugins exposes their gauges too. When a node is
removed, or turns out to be invalid, its label set is deleted from the collectors so stale series
do not linger.

If `statsd` is set, every metric is mirrored there as well, over UDP, under keys like
`core.<hostname>.deploy.count` and `pod.node.<nodename>.up`. The statsd connection is lazy and
failures are logged, never fatal.

## Authentication

Setting `auth.username` installs a unary and a stream interceptor on every rpc. The check is
deliberately simple: the request's gRPC metadata must contain an entry whose **key** is the
configured username and whose **value** is the password. There is no TLS on the core listener and
the client does not require transport security, so this is a shared secret on a trusted network,
not a public-internet authentication scheme. Put core behind something else if the network is not
trusted.

With `auth.username` empty, the API is open to anyone who can reach the port.

## Sentry

Setting `sentry_dsn` initializes the Sentry client. Anything logged at error or fatal level is
reported with its stack and, where core knows them, the caller's address and trace ID as tags.
Goroutines spawned through core's own helper capture panics to Sentry before re-raising them, and
the process flushes on exit. Empty means no Sentry.

## Profiling

The same `profile` listener serves net/http/pprof:

```shell
go tool pprof http://<core>:12346/debug/pprof/profile?seconds=30
go tool pprof http://<core>:12346/debug/pprof/heap
curl http://<core>:12346/debug/pprof/goroutine?debug=2
```

Core also sets `GOTRACEBACK=crash` in its systemd unit and raises `LimitCORE`, so a panic leaves a
core dump behind.

## Shutdown

On `SIGINT`, `SIGTERM` or `SIGQUIT` core stops accepting new work, unregisters its
`/services/{addr}` key so clients stop routing to it, gracefully stops the gRPC server, and then
waits for in-flight streaming tasks to finish before exiting. Long calls — a build, a
`RunAndWait`, a log stream — hold shutdown open, which is why the systemd unit allows 1200
seconds.

## Instance identity

Each instance's identifier is the SHA-256 of its marshalled config, and every workload it creates
is labelled `eru.coreid` with that value. Instances sharing a config share an identity by design —
it identifies the *cluster configuration* a workload belongs to, not the process that created it.
`Info` returns it.
