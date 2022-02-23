package types

import (
	"crypto/sha1" // #nosec
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	// Etcd .
	Etcd = "etcd"
	// Redis .
	Redis = "redis"
)

// Config holds eru-core config
type Config struct {
	LogLevel            string        `yaml:"log_level" required:"true" default:"INFO"`
	Bind                string        `yaml:"bind" required:"true" default:"5001"`           // HTTP API address
	LockTimeout         time.Duration `yaml:"lock_timeout" required:"true" default:"30s"`    // timeout for lock (ttl)
	GlobalTimeout       time.Duration `yaml:"global_timeout" required:"true" default:"300s"` // timeout for remove, run_and_wait and build, in second
	ConnectionTimeout   time.Duration `yaml:"connection_timeout" default:"10s"`              // timeout for connections
	HAKeepaliveInterval time.Duration `yaml:"ha_keepalive_interval" default:"16s"`           // interval for node status watcher
	Statsd              string        `yaml:"statsd"`                                        // statsd host and port
	Profile             string        `yaml:"profile"`                                       // profile ip:port
	CertPath            string        `yaml:"cert_path"`                                     // docker cert files path
	MaxConcurrency      int64         `yaml:"max_concurrency" default:"20"`                  // concurrently call single runtime in the same time
	Store               string        `yaml:"store" default:"etcd"`                          // store type

	ResourcePluginsDir     string        `yaml:"resource_plugins_dir" default:"/etc/eru/resource_plugins"` // resource plugins path
	ResourcePluginsTimeout time.Duration `yaml:"resource_plugins_timeout" default:"30s"`                   // timeout for calling resource plugins

	Auth       AuthConfig `yaml:"auth"` // grpc auth
	GRPCConfig GRPCConfig `yaml:"grpc"` // grpc config

	WALFile        string        `yaml:"wal_file" required:"true" default:"core.wal"`   // WAL file path
	WALOpenTimeout time.Duration `yaml:"wal_open_timeout" required:"true" default:"8s"` // timeout for opening a WAL file

	Git       GitConfig     `yaml:"git"`
	Etcd      EtcdConfig    `yaml:"etcd"`
	Redis     RedisConfig   `yaml:"redis"`
	Docker    DockerConfig  `yaml:"docker"`
	Scheduler SchedConfig   `yaml:"scheduler"`
	Virt      VirtConfig    `yaml:"virt"`
	Systemd   SystemdConfig `yaml:"systemd"`
	SentryDSN string        `yaml:"sentry_dsn"`
}

// Identifier returns the id of this config
// we consider the same storage as the same config
func (c Config) Identifier() string {
	s := strings.Builder{}
	_, _ = s.WriteString(c.Store)
	for _, e := range c.Etcd.Machines {
		_, _ = s.WriteString(e)
	}
	_, _ = s.WriteString(c.Etcd.Prefix)
	_, _ = s.WriteString(c.Redis.Addr)
	_, _ = s.WriteString(strconv.Itoa(c.Redis.DB))
	h := sha1.New() // #nosec
	_, _ = h.Write([]byte(s.String()))
	return fmt.Sprintf("%x", h.Sum(nil))
}

// EtcdConfig holds eru-core etcd config
type EtcdConfig struct {
	Machines   []string   `yaml:"machines" required:"true"`                           // etcd cluster addresses
	Prefix     string     `yaml:"prefix" required:"true" default:"/eru"`              // etcd lock prefix, all locks will be created under this dir
	LockPrefix string     `yaml:"lock_prefix" required:"true" default:"__lock__/eru"` // etcd lock prefix, all locks will be created under this dir
	Ca         string     `yaml:"ca"`                                                 // etcd ca
	Key        string     `yaml:"key"`                                                // etcd key
	Cert       string     `yaml:"cert"`                                               // etcd trusted_ca
	Auth       AuthConfig `yaml:"auth"`                                               // etcd auth
}

// RedisConfig holds redis config
// LockPrefix is used for lock
type RedisConfig struct {
	Addr       string `yaml:"addr" default:"localhost:6379"` // redis address
	DB         int    `yaml:"db" default:"0"`                // redis db
	LockPrefix string `yaml:"lock_prefix" default:"/lock"`   // redis lock prefix
}

// GitConfig holds eru-core git config
type GitConfig struct {
	SCMType      string        `yaml:"scm_type"`                                     // source code manager type [gitlab/github]
	PrivateKey   string        `yaml:"private_key"`                                  // private key to clone code
	Token        string        `yaml:"token"`                                        // token to call SCM API
	CloneTimeout time.Duration `yaml:"clone_timeout" required:"true" default:"300s"` // clone timeout
}

// DockerConfig holds eru-core docker config
type DockerConfig struct {
	APIVersion  string                `yaml:"version" required:"true" default:"1.32"`      // docker API version
	NetworkMode string                `yaml:"network_mode" required:"true" default:"host"` // docker network mode
	Hub         string                `yaml:"hub"`                                         // docker hub address
	Namespace   string                `yaml:"namespace"`                                   // docker hub prefix, will be set to $Hub/$HubPrefix/$appname
	BuildPod    string                `yaml:"build_pod"`                                   // podname used to build
	UseLocalDNS bool                  `yaml:"local_dns"`                                   // use node IP as dns
	Log         LogConfig             `yaml:"log"`                                         // docker log driver
	AuthConfigs map[string]AuthConfig `yaml:"auths"`                                       // docker registry credentials
}

// VirtConfig holds yavirtd config
type VirtConfig struct {
	APIVersion string `yaml:"version"` // Yavirtd API version
}

// SystemdConfig is systemd config
type SystemdConfig struct {
	Runtime string `yaml:"runtime" default:"io.containerd.eru.v2"`
}

// LogConfig define log type
type LogConfig struct {
	Type   string            `yaml:"type" required:"true" default:"journald"` // Log type, can be "journald", "json-file", "none"
	Config map[string]string `yaml:"config"`                                  // Log configs
}

// SchedConfig holds scheduler config
type SchedConfig struct {
	MaxShare       int `yaml:"maxshare" required:"true" default:"-1"`             // comlpex scheduler use maxshare
	ShareBase      int `yaml:"sharebase" required:"true" default:"100"`           // how many pieces for one core
	MaxDeployCount int `yaml:"max_deploy_count" required:"false" default:"10000"` // max deploy count of each node
}

// AuthConfig contains authorization information for connecting to a Registry
// Basically copied from https://github.com/moby/moby/blob/16a1736b9b93e44c898f95d670bbaf20a558103d/api/types/auth.go#L4
// But use yaml instead of json
// And we use it as grpc simple auth
type AuthConfig struct {
	Username string `yaml:"username,omitempty" json:"username,omitempty"`
	Password string `yaml:"password,omitempty" json:"password,omitempty"`
}

// GRPCConfig indicate grpc config
type GRPCConfig struct {
	MaxConcurrentStreams         int           `yaml:"max_concurrent_streams,omitempty" json:"max_concurrent_streams,omitempty" required:"true" default:"100"`
	MaxRecvMsgSize               int           `yaml:"max_recv_msg_size,omitempty" json:"max_recv_msg_size,omitempty" required:"true" default:"20971520"`
	ServiceDiscoveryPushInterval time.Duration `yaml:"service_discovery_interval" required:"true" default:"15s"`
	ServiceHeartbeatInterval     time.Duration `yaml:"service_heartbeat_interval" required:"true" default:"15s"`
}
