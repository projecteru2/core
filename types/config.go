package types

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"
)

const (
	Etcd  = "etcd"
	Redis = "redis"
)

type Config struct {
	Bind                string        `yaml:"bind" required:"true" default:"5001"`           // listen address
	LockTimeout         time.Duration `yaml:"lock_timeout" required:"true" default:"30s"`    // lock ttl
	GlobalTimeout       time.Duration `yaml:"global_timeout" required:"true" default:"300s"` // timeout for remove, run_and_wait and build
	ConnectionTimeout   time.Duration `yaml:"connection_timeout" required:"true" default:"10s"`
	HAKeepaliveInterval time.Duration `yaml:"ha_keepalive_interval" required:"true" default:"16s"` // interval for node status watcher
	Statsd              string        `yaml:"statsd"`                                              // statsd host:port
	Profile             string        `yaml:"profile"`                                             // profile ip:port
	CertPath            string        `yaml:"cert_path"`                                           // docker cert files path
	MaxConcurrency      int           `yaml:"max_concurrency" default:"100000"`                    // max concurrent calls to one runtime
	Store               string        `yaml:"store" default:"etcd"`
	SentryDSN           string        `yaml:"sentry_dsn"`
	ProbeTarget         string        `yaml:"probe_target" required:"false" default:"8.8.8.8:80"` // for getting outbound address

	WALFile        string        `yaml:"wal_file" required:"true" default:"core.wal"`
	WALOpenTimeout time.Duration `yaml:"wal_open_timeout" required:"true" default:"8s"`

	Auth           AuthConfig           `yaml:"auth"` // grpc auth
	GRPCConfig     GRPCConfig           `yaml:"grpc"`
	Git            GitConfig            `yaml:"git"`
	Etcd           EtcdConfig           `yaml:"etcd"`
	Redis          RedisConfig          `yaml:"redis"`
	Docker         DockerConfig         `yaml:"docker"`
	Virt           VirtConfig           `yaml:"virt"`
	Systemd        SystemdConfig        `yaml:"systemd"`
	Scheduler      SchedulerConfig      `yaml:"scheduler"`
	ResourcePlugin ResourcePluginConfig `yaml:"resource_plugin"`
	Log            ServerLogConfig      `yaml:"log"`
}

// Identifier returns a sha256 over the fields that identify the backing store.
func (c Config) Identifier() (string, error) {
	b, err := json.Marshal(struct {
		Store      string
		Machines   []string
		EtcdPrefix string
		RedisAddr  string
		RedisDB    int
	}{c.Store, c.Etcd.Machines, c.Etcd.Prefix, c.Redis.Addr, c.Redis.DB})
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", sha256.Sum256(b)), nil
}

// AuthConfig holds registry credentials, also reused for grpc basic auth.
type AuthConfig struct {
	Username string `yaml:"username,omitempty" json:"username,omitempty"`
	Password string `yaml:"password,omitempty" json:"password,omitempty"`
}

type GRPCConfig struct {
	MaxConcurrentStreams         uint32        `yaml:"max_concurrent_streams,omitempty" json:"max_concurrent_streams,omitempty" required:"true" default:"100"`
	MaxRecvMsgSize               int           `yaml:"max_recv_msg_size,omitempty" json:"max_recv_msg_size,omitempty" required:"true" default:"20971520"`
	ServiceDiscoveryPushInterval time.Duration `yaml:"service_discovery_interval" required:"true" default:"15s"`
	ServiceHeartbeatInterval     time.Duration `yaml:"service_heartbeat_interval" required:"true" default:"15s"`
}

type GitConfig struct {
	SCMType      string        `yaml:"scm_type"`    // source code manager type [gitlab/github]
	PrivateKey   string        `yaml:"private_key"` // private key to clone code
	Token        string        `yaml:"token"`       // token to call SCM API
	CloneTimeout time.Duration `yaml:"clone_timeout" default:"300s"`
}

type EtcdConfig struct {
	Machines   []string   `yaml:"machines" required:"true"`
	Prefix     string     `yaml:"prefix" required:"true" default:"/eru"`              // key prefix for core data
	LockPrefix string     `yaml:"lock_prefix" required:"true" default:"__lock__/eru"` // key prefix for locks
	Ca         string     `yaml:"ca"`
	Key        string     `yaml:"key"`
	Cert       string     `yaml:"cert"`
	Auth       AuthConfig `yaml:"auth"`
}

type RedisConfig struct {
	Addr       string `yaml:"addr" default:"localhost:6379"`
	LockPrefix string `yaml:"lock_prefix" default:"/lock"`
	DB         int    `yaml:"db" default:"0"`
}

type DockerConfig struct {
	APIVersion  string    `yaml:"version" required:"true" default:"1.32"`
	NetworkMode string    `yaml:"network_mode" required:"true" default:"host"`
	UseLocalDNS bool      `yaml:"use_local_dns"` // use node IP as dns
	Log         LogConfig `yaml:"log"`           // docker log driver

	Hub         string                `yaml:"hub"`
	Namespace   string                `yaml:"namespace"` // image path becomes $Hub/$Namespace/$appname
	BuildPod    string                `yaml:"build_pod"` // podname used to build
	AuthConfigs map[string]AuthConfig `yaml:"auths"`     // docker registry credentials
}

type VirtConfig struct {
	APIVersion string `yaml:"version" default:"v1"` // Yavirtd API version
}

type SystemdConfig struct {
	Runtime string `yaml:"runtime" default:"io.containerd.eru.v2"`
}

type SchedulerConfig struct {
	MaxShare       int `yaml:"maxshare" required:"true" default:"-1"`
	ShareBase      int `yaml:"sharebase" required:"true" default:"100"` // how many pieces for one core
	MaxDeployCount int `yaml:"max_deploy_count" default:"10000"`
}

type ResourcePluginConfig struct {
	Dir         string        `yaml:"dir" default:""`
	CallTimeout time.Duration `yaml:"call_timeout" default:"30s"`
	Whitelist   []string      `yaml:"whitelist"`
}

type LogConfig struct {
	Type   string            `yaml:"type" required:"true" default:"journald"` // journald, json-file or none
	Config map[string]string `yaml:"config"`
}

type ServerLogConfig struct {
	Level   string `yaml:"level" default:"info"`
	UseJSON bool   `yaml:"use_json"`
	// file log only
	Filename   string `yaml:"filename"`
	MaxSize    int    `yaml:"maxsize" default:"500"`
	MaxAge     int    `yaml:"max_age" default:"28"`
	MaxBackups int    `yaml:"max_backups" default:"3"`
}
