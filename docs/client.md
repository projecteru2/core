# Go client

`github.com/projecteru2/core/client` is the Go client other services use to talk to core. It is
a thin wrapper around the generated `pb.CoreRPCClient` plus the pieces that make a multi-instance
deployment usable: address resolution, load balancing, retry of the watch streams, and
authentication.

## A single connection

```go
import (
    "github.com/projecteru2/core/client"
    pb "github.com/projecteru2/core/rpc/gen"
    "github.com/projecteru2/core/types"
)

cli, err := client.NewClient(ctx, "127.0.0.1:5001", types.AuthConfig{})
if err != nil {
    return err
}
rpc := cli.GetRPCClient()

info, err := rpc.Info(ctx, &pb.Empty{})
```

`GetRPCClient()` returns the generated `CoreRPCClient`. The connection is
configured with insecure transport credentials, a 6-minute keepalive ping with a 1-second
timeout, and `round_robin` load balancing across whatever the resolver produced.

If core requires auth, pass the same username and password as its `auth` config; the client
installs per-RPC credentials that send them as one metadata pair. Note this travels in the clear —
the client does not enable TLS.

## Address forms

Importing `client` registers two gRPC resolvers, so `addr` may be:

| Target | Meaning |
| --- | --- |
| `host:port` | one instance, no resolution |
| `static://_/addr1,addr2,addr3` | a fixed set of instances, round-robined |
| `eru:///addr` | bootstrap from one instance, then follow service discovery |

`eru://` is the interesting one: the client connects to the given address, subscribes to
`WatchServiceStatus`, and rewrites the connection's address list every time core pushes a new set.
Instances that come up join the round-robin pool, instances that go away leave it — no restart, no
config change. Credentials are passed as the `types.AuthConfig` argument to `NewClient`, not in the URL.

## Connection pool

For callers that would rather hold several independent connections and pick a live one:

```go
pool, err := client.NewCoreRPCClientPool(ctx, &client.PoolConfig{
    EruAddrs:          []string{"10.0.0.1:5001", "10.0.0.2:5001"},
    Auth:              types.AuthConfig{},
    ConnectionTimeout: 10 * time.Second,
})
if err != nil {
    return err
}

rpc := pool.GetClient()
```

The pool dials every address at construction, drops the ones that fail, and returns
`ErrAllConnectionsFailed` if none answered. A background loop re-probes every client with `Info`
every `2 × ConnectionTimeout`; `GetClient` returns the first client currently marked alive, or the
first client at all if everything is down.

## Watching service status directly

If you only want the address list — to feed your own balancer, say:

```go
import "github.com/projecteru2/core/client/servicediscovery"

sd := servicediscovery.New("127.0.0.1:5001", types.AuthConfig{})
ch, err := sd.Watch(ctx)
for addresses := range ch {
    // current set of live core instances
}
```

`Watch` reconnects on its own: if the stream dies it retries after 10 seconds, and if no push
arrives within the interval core advertised (twice `grpc.service_discovery_interval`) it cancels
and re-establishes the watch rather than sitting on a silent connection.

## Retries

The client installs both a unary and a stream retry interceptor. Both default to
`Max: 0` — no retries — because a resource operation is not safe to replay blindly.

The stream interceptor only wraps two methods, which are pure watches and therefore replayable:

- `/pb.CoreRPC/WatchServiceStatus`
- `/pb.CoreRPC/WorkloadStatusStream`

For those, a broken stream is re-established with exponential backoff and the last sent message is
replayed. The service-discovery client raises the limit to 1 for its own watch.
