package embedded

import (
	"errors"
	"net/url"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/namespace"
	"go.etcd.io/etcd/server/v3/embed"
	"go.etcd.io/etcd/server/v3/etcdserver/api/v3client"
)

const startTimeout = 30 * time.Second

var errStartTimeout = errors.New("embedded etcd start timeout")

// Cluster is a single-member etcd server running inside the process.
type Cluster struct {
	etcd *embed.Etcd
}

// New starts an embedded etcd storing its data under dir.
func New(dir string) (*Cluster, error) {
	cfg := embed.NewConfig()
	cfg.Dir = dir
	cfg.LogLevel = "error"
	cfg.TickMs = 10
	cfg.ElectionMs = 100
	cfg.ListenClientUrls = nil
	cfg.AdvertiseClientUrls = nil
	cfg.ListenPeerUrls = nil
	cfg.AdvertisePeerUrls = []url.URL{{Scheme: "http", Host: "localhost:0"}}
	cfg.InitialCluster = cfg.InitialClusterFromName(cfg.Name)
	etcd, err := embed.StartEtcd(cfg)
	if err != nil {
		return nil, err
	}
	select {
	case <-etcd.Server.ReadyNotify():
	case <-time.After(startTimeout):
		etcd.Close()
		return nil, errStartTimeout
	}
	return &Cluster{etcd: etcd}, nil
}

// Client returns an in-process client scoped to prefix.
func (c *Cluster) Client(prefix string) *clientv3.Client {
	cli := v3client.New(c.etcd.Server)
	cli.KV = namespace.NewKV(cli.KV, prefix)
	cli.Watcher = namespace.NewWatcher(cli.Watcher, prefix)
	cli.Lease = namespace.NewLease(cli.Lease, prefix)
	return cli
}

// Close stops the server.
func (c *Cluster) Close() {
	c.etcd.Close()
}
