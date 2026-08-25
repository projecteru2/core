# Configuration

Core reads one YAML file, given by `--config` or `ERU_CONFIG_PATH` (default `/etc/eru/core.yaml`).
The file is loaded with [configor](https://github.com/jinzhu/configor): keys that are absent fall
back to the `default` declared on the field in `types/config.go`, and unknown keys are ignored.

[`core.yaml.sample`](https://github.com/projecteru2/core/blob/master/core.yaml.sample) in the repo
root is a working starting point. Every key below is grouped the way that file groups them.

Durations are Go duration strings (`30s`, `5m`).

## Server

| Key | Type | Default | Meaning |
| --- | --- | --- | --- |
| `bind` | string | `5001` | gRPC listen address. Must be `host:port` — use `:5001` to listen on all interfaces |
| `lock_timeout` | duration | `30s` | TTL of every distributed lock core takes (pod, node, workload) |
| `global_timeout` | duration | `300s` | Deadline for long transactions: deploy, realloc, remove, remap, the metrics scrape |
| `connection_timeout` | duration | `10s` | Engine connect/ping deadline, and the interval between engine-cache liveness sweeps |
| `ha_keepalive_interval` | duration | `16s` | TTL of the node-status-watcher election key; see [Operations](operations.md) |
| `statsd` | string | — | `host:port` of a statsd server. Empty disables statsd; Prometheus is unaffected |
| `profile` | string | — | `host:port` for the HTTP server exposing `/metrics` and net/http/pprof. Empty disables it |
| `cert_path` | string | — | Directory used to materialize per-node TLS material before handing it to the engine client. Empty means plain HTTP |
| `max_concurrency` | int | `100000` | Size of the goroutine pools core uses for fan-out work |
| `store` | string | `etcd` | Metadata backend: `etcd` or `redis` |
| `sentry_dsn` | string | — | Sentry DSN. Empty disables Sentry |
| `probe_target` | string | `8.8.8.8:80` | UDP dial target used to learn core's own outbound address when `bind` has no explicit IP |
| `wal_file` | string | `core.wal` | Path of the local bbolt write-ahead log |
| `wal_open_timeout` | duration | `8s` | How long to wait for the WAL file lock on start |

## Auth

`auth` is the credential the gRPC server requires. If `username` is empty, core installs no auth
interceptor and the API is open.

```yaml
auth:
    username: admin
    password: password
```

| Key | Type | Default | Meaning |
| --- | --- | --- | --- |
| `auth.username` | string | — | Metadata key clients must send |
| `auth.password` | string | — | Expected value of that metadata key |

## gRPC

| Key | Type | Default | Meaning |
| --- | --- | --- | --- |
| `grpc.max_concurrent_streams` | int | `100` | `grpc.MaxConcurrentStreams` on the server |
| `grpc.max_recv_msg_size` | int | `20971520` | Max received message size, in bytes (20 MiB) |
| `grpc.service_discovery_interval` | duration | `15s` | How often `WatchServiceStatus` re-pushes the instance list. Values under 1s are replaced by 15s. Clients are told to expect a push at most every 2× this |
| `grpc.service_heartbeat_interval` | duration | `15s` | TTL and refresh period of this instance's `/services/<addr>` registration |

## Store — etcd

Used when `store` is `etcd` (the default). Ignored when `--embedded-storage` supplies an
in-process cluster, except for `prefix`, which still namespaces the keys.

| Key | Type | Default | Meaning |
| --- | --- | --- | --- |
| `etcd.machines` | list of string | — (required) | etcd client endpoints |
| `etcd.prefix` | string | `/eru` | Namespace prepended to every key core reads or writes |
| `etcd.lock_prefix` | string | `__lock__/eru` | Namespace for lock keys |
| `etcd.ca` | string | — | Path to the CA file. TLS is enabled only when `ca`, `cert` and `key` are all set |
| `etcd.cert` | string | — | Path to the client certificate |
| `etcd.key` | string | — | Path to the client key |
| `etcd.auth.username` | string | — | etcd username |
| `etcd.auth.password` | string | — | etcd password |

## Store — redis

Used when `store` is `redis`. Core relies on redis keyspace notifications for its watch streams,
so the server must have them enabled.

| Key | Type | Default | Meaning |
| --- | --- | --- | --- |
| `redis.addr` | string | `localhost:6379` | Redis address |
| `redis.lock_prefix` | string | `/lock` | Prefix for lock keys |
| `redis.db` | int | `0` | Redis database index; also the db number in the keyspace-notification channel |

## Git / SCM

Only needed for `BuildImage` with the SCM build method. If `scm_type` is unset, core logs a
warning at startup and the build API returns an error.

| Key | Type | Default | Meaning |
| --- | --- | --- | --- |
| `git.scm_type` | string | — | `github` or `gitlab`; anything else disables the build API |
| `git.private_key` | string | — | Path to the SSH private key used to clone |
| `git.token` | string | — | API token, sent as the `Authorization` header when fetching artifacts |
| `git.clone_timeout` | duration | `300s` | Clone deadline |

## SSH

Core's own key pair for the nodes it drives over SSH (`process://`). The key is per core, not per
node.

| Key | Type | Default | Meaning |
| --- | --- | --- | --- |
| `ssh.private_key` | string | — | Path to the private key core authenticates with |
| `ssh.user` | string | `root` | Login user, overridden by a `user@` prefix in the node endpoint |
| `ssh.known_hosts` | string | — | Path to a `known_hosts` file. Empty accepts any host key |

## Docker

| Key | Type | Default | Meaning |
| --- | --- | --- | --- |
| `docker.network_mode` | string | `host` | Network mode for workloads that do not request a network |
| `docker.use_local_dns` | bool | `false` | When the deploy request sets no DNS, use the node's own IP as the workload's resolver |
| `docker.log.type` | string | `journald` | Default log driver for workloads (`journald`, `json-file`, `none`, …) |
| `docker.log.config` | map of string | — | Extra options passed to that log driver |
| `docker.hub` | string | — | Registry host used when building image references |
| `docker.namespace` | string | — | Path segment between host and app name: `hub/namespace/appname:tag` |
| `docker.auths` | map | — | Registry credentials, keyed by registry host: `{username, password}` |

Core always forces `mode=non-blocking`, `max-buffer-size=4m` and a per-workload `tag` into the
log driver options before merging `docker.log.config`.

## Build

| Key | Type | Default | Meaning |
| --- | --- | --- | --- |
| `build.node_filter.podname` | string | — | Build only in this pod |
| `build.node_filter.includes` | list of string | — | Build only on these nodes |
| `build.node_filter.excludes` | list of string | — | Never build on these nodes |
| `build.node_filter.labels` | map of string | — | Build only on nodes carrying these labels |
| `build.node_filter.all` | bool | `false` | Also consider nodes that are down or bypassed |

`BuildImage` picks the most idle node of that selection. A request may carry its own
`node_filter`, which is intersected into the configured one: naming a pod or a label value the
config rules out is rejected with `ErrInvaildNodeFilter`, and `all` is never taken from the
request. With nothing configured, every node is a candidate.

## Process

| Key | Type | Default | Meaning |
| --- | --- | --- | --- |
| `process.root` | string | `/var/lib/eru/process` | Node directory holding the per-workload overlays and the artifact cache |

## Virt (yavirt)

| Key | Type | Default | Meaning |
| --- | --- | --- | --- |
| `virt.version` | string | `v1` | yavirtd API version |

## Scheduler

These feed the built-in `cpumem` resource plugin.

| Key | Type | Default | Meaning |
| --- | --- | --- | --- |
| `scheduler.sharebase` | int | `100` | How many pieces one CPU core is divided into, i.e. the resolution of fractional CPU requests |
| `scheduler.maxshare` | int | `-1` | How many cores may be used as shared (fragmented) cores; `-1` means no limit |
| `scheduler.max_deploy_count` | int | `10000` | Declared in the config struct, but no code in core reads it today |

## Resource plugins

| Key | Type | Default | Meaning |
| --- | --- | --- | --- |
| `resource_plugin.dir` | string | — | Directory scanned for plugins. Empty means only the built-in `cpumem` plugin is loaded |
| `resource_plugin.call_timeout` | duration | `30s` | Deadline for a single binary-plugin invocation |
| `resource_plugin.whitelist` | list of string | — | If set, only these plugin names are consulted when reading or fixing node resource info (`GetNodeResource`, `GetPodResource`). Allocation always consults every loaded plugin |

See [Resource plugins](resource-plugins.md) for the contract these files must satisfy.

## Log

| Key | Type | Default | Meaning |
| --- | --- | --- | --- |
| `log.level` | string | `info` | zerolog level: `trace`, `debug`, `info`, `warn`, `error`, … |
| `log.use_json` | bool | `false` | Emit JSON on stdout instead of the console writer. Ignored when `log.filename` is set — file logs are always JSON |
| `log.filename` | string | — | Log to this file with rotation instead of stdout |
| `log.maxsize` | int | `500` | Rotate after this many megabytes |
| `log.max_age` | int | `28` | Keep rotated files this many days |
| `log.max_backups` | int | `3` | Keep at most this many rotated files |

## Keys the sample still shows but core no longer reads

`core.yaml.sample` predates two renames. Both are silently ignored if you copy them verbatim:

- top-level `log_level` — use the `log:` section above (`log.level`)
- `docker.auth` (single credential) — use `docker.auths`, a map keyed by registry host
- `docker.build_pod` — use `build.node_filter`
- the `systemd:` section — the engine it configured is gone; see `process` above
