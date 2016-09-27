package types

type Config struct {
	Bind           string   `yaml:"bind"`             // HTTP API address
	AgentPort      string   `yaml:"agent_port"`       // Agent HTTP port, may not be used
	PermDir        string   `yaml:"permdir"`          // Permanent dir on host
	EtcdMachines   []string `yaml:"etcd"`             // etcd cluster addresses
	EtcdLockPrefix string   `yaml:"etcd_lock_prefix"` // etcd lock prefix, all locks will be created under this dir
	ResourceAlloc  string   `yaml:"resource_alloc`    // scheduler or cpu-period TODO give it a good name

	Git       GitConfig    `yaml:"git"`
	Docker    DockerConfig `yaml:"docker"`
	Scheduler SchedConfig  `yaml:"scheduler"`
}

type GitConfig struct {
	PublicKey   string `yaml:"public_key"`   // public key to clone code
	PrivateKey  string `yaml:"private_key"`  // private key to clone code
	GitlabToken string `yaml:"gitlab_token"` // GitLab token to call GitLab API
}

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

type SchedConfig struct {
	LockKey string `yaml:"lock_key"` // key for etcd lock
	LockTTL int    `yaml:"lock_ttl"` // TTL for etcd lock
	Type    string `yaml:"type"`     // choose simple or complex scheduler
}
