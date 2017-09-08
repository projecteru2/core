package types

import "time"

// Config holds eru-core config
type Config struct {
	LogLevel       string        `yaml:"log_level"`
	Bind           string        `yaml:"bind"`             // HTTP API address
	AppDir         string        `yaml:"appdir"`           // App directory inside container
	PermDir        string        `yaml:"permdir"`          // Permanent dir on host
	BackupDir      string        `yaml:"backupdir"`        // Backup dir on host
	EtcdMachines   []string      `yaml:"etcd"`             // etcd cluster addresses
	EtcdLockPrefix string        `yaml:"etcd_lock_prefix"` // etcd lock prefix, all locks will be created under this dir
	Statsd         string        `yaml:"statsd"`           // Statsd host and port
	Zone           string        `yaml:"zone"`             // zone for core, e.g. C1, C2
	ImageCache     int           `yaml:"image_cache"`      // cache image count
	LockTimeout    int           `yaml:"lock_timeout"`     // timeout for lock (ttl)
	GlobalTimeout  time.Duration `yaml:"global_timeout"`   // timeout for remove, run_and_wait and build, in second

	Git       GitConfig    `yaml:"git"`
	Docker    DockerConfig `yaml:"docker"`
	Scheduler SchedConfig  `yaml:"scheduler"`
	Syslog    SyslogConfig `yaml:"syslog"`
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
	APIVersion  string `yaml:"version"`      // docker API version
	LogDriver   string `yaml:"log_driver"`   // docker log driver, can be "json-file", "none"
	NetworkMode string `yaml:"network_mode"` // docker network mode
	CertPath    string `yaml:"cert_path"`    // docker cert files path
	Hub         string `yaml:"hub"`          // docker hub address
	HubPrefix   string `yaml:"hub_prefix"`   // docker hub prefix, will be set to $Hub/$HubPrefix/$appname
	BuildPod    string `yaml:"build_pod"`    // podname used to build
	UseLocalDNS bool   `yaml:"local_dns"`    // use node IP as dns
}

// SchedConfig holds scheduler config
type SchedConfig struct {
	MaxShare  int64 `yaml:"maxshare"`  // comlpex scheduler use maxshare
	ShareBase int64 `yaml:"sharebase"` // how many pieces for one core
}

// SyslogConfig 用于debug模式容器的日志收集
type SyslogConfig struct {
	Address  string `yaml:"address"`
	Facility string `yaml:"facility"`
	Format   string `yaml:"format"`
}
