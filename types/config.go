package types

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const (
	Etcd  = "etcd"
	Redis = "redis"
)

type Config struct {
	Bind                string        `yaml:"bind" required:"true" default:"5001"`
	LockTimeout         time.Duration `yaml:"lock_timeout" required:"true" default:"30s"`
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
	SSH            SSHConfig            `yaml:"ssh"`
	Etcd           EtcdConfig           `yaml:"etcd"`
	Redis          RedisConfig          `yaml:"redis"`
	Docker         DockerConfig         `yaml:"docker"`
	Process        ProcessConfig        `yaml:"process"`
	Virt           VirtConfig           `yaml:"virt"`
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
	Prefix     string     `yaml:"prefix" required:"true" default:"/eru"` // key prefix for core data
	LockPrefix string     `yaml:"lock_prefix" required:"true" default:"__lock__/eru"`
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

// SSHConfig is core's key pair for nodes it drives over SSH.
type SSHConfig struct {
	PrivateKey string `yaml:"private_key"` // file path
	User       string `yaml:"user" default:"root"`
	KnownHosts string `yaml:"known_hosts"` // file path; empty accepts any host key
}

// ProcessConfig is the node-side layout the process engine writes into.
type ProcessConfig struct {
	Root string `yaml:"root" default:"/var/lib/eru/process"`
}

type DockerConfig struct {
	NetworkMode string    `yaml:"network_mode" required:"true" default:"host"`
	UseLocalDNS bool      `yaml:"use_local_dns"` // use node IP as dns
	Log         LogConfig `yaml:"log"`           // docker log driver

	Hub         string                `yaml:"hub"`
	Namespace   string                `yaml:"namespace"` // image path becomes $Hub/$Namespace/$appname
	BuildPod    string                `yaml:"build_pod"` // podname used to build
	AuthConfigs map[string]AuthConfig `yaml:"auths"`     // docker registry credentials
}

// ImageTag renders the registry reference an app's built image is pushed under.
func (c DockerConfig) ImageTag(appname, tag string) string {
	prefix := strings.Trim(c.Namespace, "/")
	if prefix == "" {
		return fmt.Sprintf("%s/%s:%s", c.Hub, appname, tag)
	}
	return fmt.Sprintf("%s/%s/%s:%s", c.Hub, prefix, appname, tag)
}

type VirtConfig struct {
	APIVersion string `yaml:"version" default:"v1"` // Yavirtd API version
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
	Level      string `yaml:"level" default:"info"`
	UseJSON    bool   `yaml:"use_json"`
	Filename   string `yaml:"filename"`
	MaxSize    int    `yaml:"maxsize" default:"500"`
	MaxAge     int    `yaml:"max_age" default:"28"`
	MaxBackups int    `yaml:"max_backups" default:"3"`
}
