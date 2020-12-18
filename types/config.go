package types

import (
	"time"
)

// Config holds eru-core config
type Config struct {
	LogLevel      string        `yaml:"log_level" required:"true" default:"INFO"`
	Bind          string        `yaml:"bind" required:"true" default:"5001"`           // HTTP API address
	LockTimeout   time.Duration `yaml:"lock_timeout" required:"true" default:"30s"`    // timeout for lock (ttl)
	GlobalTimeout time.Duration `yaml:"global_timeout" required:"true" default:"300s"` // timeout for remove, run_and_wait and build, in second
	Statsd        string        `yaml:"statsd"`                                        // statsd host and port
	Profile       string        `yaml:"profile"`                                       // profile ip:port
	CertPath      string        `yaml:"cert_path"`                                     // docker cert files path
	Auth          AuthConfig    `yaml:"auth"`                                          // grpc auth
	GRPCConfig    GRPCConfig    `yaml:"grpc"`                                          // grpc config

	WALFile        string        `yaml:"wal_file" required:"true" default:"core.wal"`   // WAL file path
	WALOpenTimeout time.Duration `yaml:"wal_open_timeout" required:"true" default:"8s"` // timeout for opening a WAL file

	Git       GitConfig     `yaml:"git"`
	Etcd      EtcdConfig    `yaml:"etcd"`
	Docker    DockerConfig  `yaml:"docker"`
	Scheduler SchedConfig   `yaml:"scheduler"`
	Virt      VirtConfig    `yaml:"virt"`
	Systemd   SystemdConfig `yaml:"systemd"`
	SentryDSN string        `yaml:"sentry_dsn"`
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
	Username string `yaml:"username" default:"root"`
}

// LogConfig define log type
type LogConfig struct {
	Type   string            `yaml:"type" required:"true" default:"journald"` // Log type, can be "journald", "json-file", "none"
	Config map[string]string `yaml:"config"`                                  // Log configs
}

// SchedConfig holds scheduler config
type SchedConfig struct {
	MaxShare  int `yaml:"maxshare" required:"true" default:"-1"`   // comlpex scheduler use maxshare
	ShareBase int `yaml:"sharebase" required:"true" default:"100"` // how many pieces for one core
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
