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

	defaultVersion = "latest"
)

type Config struct {
	Bind                string        `yaml:"bind" required:"true" default:"5001"`
	LockTimeout         time.Duration `yaml:"lock_timeout" required:"true" default:"30s"`
	GlobalTimeout       time.Duration `yaml:"global_timeout" required:"true" default:"300s"` // timeout for remove, run_and_wait and build
	ConnectionTimeout   time.Duration `yaml:"connection_timeout" required:"true" default:"10s"`
	HAKeepaliveInterval time.Duration `yaml:"ha_keepalive_interval" required:"true" default:"16s"` // interval for node status watcher
	Statsd              string        `yaml:"statsd"`                                              // statsd host:port
	Profile             string        `yaml:"profile"`                                             // profile ip:port
	MaxConcurrency      int           `yaml:"max_concurrency" default:"100000"`                    // max concurrent calls to one runtime
	Store               string        `yaml:"store" default:"etcd"`
	SentryDSN           string        `yaml:"sentry_dsn"`
	ProbeTarget         string        `yaml:"probe_target" required:"false" default:"8.8.8.8:80"` // for getting outbound address

	Auth           AuthConfig           `yaml:"auth"` // grpc auth
	GRPCConfig     GRPCConfig           `yaml:"grpc"`
	Git            GitConfig            `yaml:"git"`
	SSH            SSHConfig            `yaml:"ssh"`
	Etcd           EtcdConfig           `yaml:"etcd"`
	Redis          RedisConfig          `yaml:"redis"`
	Registry       RegistryConfig       `yaml:"registry"`
	Build          BuildConfig          `yaml:"build"`
	Containerd     ContainerdConfig     `yaml:"containerd"`
	Process        ProcessConfig        `yaml:"process"`
	Cocoon         CocoonConfig         `yaml:"cocoon"`
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

// SSHConfig is core's key pair for the nodes it drives over SSH.
type SSHConfig struct {
	PrivateKey string `yaml:"private_key"` // file path
	User       string `yaml:"user" default:"root"`
	KnownHosts string `yaml:"known_hosts"` // file path; empty accepts any host key
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

// RegistryConfig is the registry every engine pulls from and pushes built images to.
type RegistryConfig struct {
	Hub       string                `yaml:"hub"`
	Namespace string                `yaml:"namespace"`  // image path becomes $Hub/$Namespace/$appname
	Auths     map[string]AuthConfig `yaml:"auths"`      // keyed by registry host
	PlainHTTP []string              `yaml:"plain_http"` // registry hosts served without TLS
}

func (c RegistryConfig) BuildRefs(appname string, tags []string) []string {
	if len(tags) == 0 {
		return []string{c.ImageTag(appname, defaultVersion)}
	}
	refs := make([]string, 0, len(tags))
	for _, tag := range tags {
		refs = append(refs, c.ImageTag(appname, tag))
	}
	return refs
}

// ImageTag renders the registry reference an app's built image is pushed under.
func (c RegistryConfig) ImageTag(appname, tag string) string {
	prefix := strings.Trim(c.Namespace, "/")
	if prefix == "" {
		return fmt.Sprintf("%s/%s:%s", c.Hub, appname, tag)
	}
	return fmt.Sprintf("%s/%s/%s:%s", c.Hub, prefix, appname, tag)
}

// BuildConfig selects the nodes allowed to run in-cluster image builds.
type BuildConfig struct {
	NodeFilter NodeFilter `yaml:"node_filter"`
}

// ContainerdConfig is the node-side layout the containerd engine reaches over SSH.
type ContainerdConfig struct {
	Socket      string        `yaml:"socket" default:"/run/containerd/containerd.sock"`
	Namespace   string        `yaml:"namespace" default:"eru"`
	BuildKit    string        `yaml:"buildkit" default:"/run/buildkit/buildkitd.sock"` // a tcp:// address is dialed directly
	StopTimeout time.Duration `yaml:"stop_timeout" default:"10s"`                      // grace period before the task is killed
}

// ProcessConfig is the node-side layout the process engine writes into.
type ProcessConfig struct {
	Root        string        `yaml:"root" default:"/var/lib/eru/process"`
	StopTimeout time.Duration `yaml:"stop_timeout" default:"10s"` // grace period before systemd kills the unit
}

// CocoonConfig is the node-side layout the cocoon engine drives over SSH.
type CocoonConfig struct {
	Binary       string `yaml:"binary" default:"cocoon"`               // the cocoon command on the node; a sudo wrapper works
	Root         string `yaml:"root" default:"/var/lib/eru/cocoon"`    // durable copies of the workload records
	RunDir       string `yaml:"run_dir" default:"/var/lib/cocoon/run"` // cocoon's run_dir, where the guest consoles live
	CgroupParent string `yaml:"cgroup_parent" default:"cocoon.slice"`  // cocoon's cgroup_parent
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
