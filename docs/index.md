# core

Eru core is a stateless gRPC resource scheduler. It keeps cluster metadata in etcd or redis,
allocates resources through pluggable resource plugins, and deploys workloads onto Docker
containers or yavirt virtual machines. Every instance is interchangeable: state lives in the
store, coordination happens through distributed locks, and clients find live instances through
the built-in service discovery.

```
cli / agent / your service
        │  gRPC (CoreRPC, rpc/gen/core.proto)
        ▼
   ┌─────────────────────────────────────────────┐
   │ eru-core                                    │
   │   rpc ──► cluster/calcium ──► strategy      │  deploy plan
   │             │      │                        │
   │             │      ├──► resource/cobalt ──► cpumem (built in)
   │             │      │                    └─► binary plugins
   │             │      └──► wal (bbolt)         │  crash recovery
   │             ▼                               │
   │           store ──► etcd | redis            │  metadata, locks, status
   │             │                               │
   │           engine/factory (cached, per node) │
   └─────────────┼───────────────────────────────┘
                 ▼
   tcp:// unix:// | virt-grpc:// | systemd:// | mock://
```

## Guides

- [Installation](installation.md) — release archives, container image, building from source,
  the systemd unit
- [Configuration](configuration.md) — every key core reads, with types and defaults
- [Architecture](architecture.md) — the packages, what each owns, and how a deploy request flows
- [gRPC API](api.md) — every rpc grouped by domain, with the key request fields
- [Engines](engines.md) — docker, virt, systemd and fake; endpoint schemes, the engine cache, TLS
- [Resource plugins](resource-plugins.md) — the plugin contract, cpumem and binary plugins
- [Deploy strategies](deploy-strategies.md) — AUTO, FILL, EACH, GLOBAL, DRAINED and when to use each
- [Storage](storage.md) — etcd key layout, the redis backend, locks, embedded etcd
- [Go client](client.md) — the client library, connection pool and service-discovery resolvers
- [Operations](operations.md) — WAL recovery, the node status watcher, metrics, auth, sentry, profiling

## Repository

Source and issue tracker: [github.com/projecteru2/core](https://github.com/projecteru2/core).
Part of the [Eru](https://github.com/projecteru2) cluster stack, alongside
[agent](https://github.com/projecteru2/agent), [cli](https://github.com/projecteru2/cli),
[resource-extend](https://github.com/projecteru2/resource-extend),
[quickstart](https://github.com/projecteru2/quickstart) and
[footstone](https://github.com/projecteru2/footstone).
The `virt` engine targets [yavirt](https://github.com/projecteru2/yavirt), which is archived.
