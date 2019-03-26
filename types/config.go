package types

import (
	"time"
)

// Config holds eru-core config
type Config struct {
	LogLevel      string        `yaml:"log_level"`
	Bind          string        `yaml:"bind"`           // HTTP API address
	Statsd        string        `yaml:"statsd"`         // statsd host and port
	Profile       string        `yaml:"profile"`        // profile ip:port
	LockTimeout   int           `yaml:"lock_timeout"`   // timeout for lock (ttl)
	GlobalTimeout time.Duration `yaml:"global_timeout"` // timeout for remove, run_and_wait and build, in second
	Auth          AuthConfig    `yaml:"auth"`           // grpc auth

	Git       GitConfig    `yaml:"git"`
	Etcd      EtcdConfig   `yaml:"etcd"`
	Docker    DockerConfig `yaml:"docker"`
	Scheduler SchedConfig  `yaml:"scheduler"`
}

// EtcdConfig holds eru-core etcd config
type EtcdConfig struct {
	Machines   []string `yaml:"machines"`    // etcd cluster addresses
	Prefix     string   `yaml:"prefix"`      // etcd lock prefix, all locks will be created under this dir
	LockPrefix string   `yaml:"lock_prefix"` // etcd lock prefix, all locks will be created under this dir
}

// GitConfig holds eru-core git config
type GitConfig struct {
	SCMType    string `yaml:"scm_type"`    // source code manager type [gitlab/github]
	PublicKey  string `yaml:"public_key"`  // public key to clone code
	PrivateKey string `yaml:"private_key"` // private key to clone code
	Token      string `yaml:"token"`       // Token to call SCM API
}

// DockerConfig holds eru-core docker config
type DockerConfig struct {
	APIVersion  string                `yaml:"version"`      // docker API version
	NetworkMode string                `yaml:"network_mode"` // docker network mode
	CertPath    string                `yaml:"cert_path"`    // docker cert files path
	Hub         string                `yaml:"hub"`          // docker hub address
	Namespace   string                `yaml:"namespace"`    // docker hub prefix, will be set to $Hub/$HubPrefix/$appname
	BuildPod    string                `yaml:"build_pod"`    // podname used to build
	UseLocalDNS bool                  `yaml:"local_dns"`    // use node IP as dns
	Log         LogConfig             `yaml:"log"`          // docker log driver
	AuthConfigs map[string]AuthConfig `yaml:"auths"`        // docker registry credentials
}

// LogConfig define log type
type LogConfig struct {
	Type   string            `yaml:"type"`   // Log type, can be "journald", "json-file", "none"
	Config map[string]string `yaml:"config"` // Log configs
}

// SchedConfig holds scheduler config
type SchedConfig struct {
	MaxShare  int `yaml:"maxshare"`  // comlpex scheduler use maxshare
	ShareBase int `yaml:"sharebase"` // how many pieces for one core
}

// AuthConfig contains authorization information for connecting to a Registry
// Basically copied from https://github.com/moby/moby/blob/16a1736b9b93e44c898f95d670bbaf20a558103d/api/types/auth.go#L4
// But use yaml instead of json
// And we use it as grpc simple auth
type AuthConfig struct {
	Username string `yaml:"username,omitempty" json:"username,omitempty"`
	Password string `yaml:"password,omitempty" json:"password,omitempty"`
}
