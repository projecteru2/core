package types

type Config struct {
	ListenAddress string   `yaml:"bind"`
	AgentPort     string   `yaml:"agent_port"`
	PermDir       string   `yaml:"permdir"`
	EtcdMachines  []string `yaml:"etcd"`

	Git    GitConfig    `yaml:"git"`
	Docker DockerConfig `yaml:"docker"`
}

type GitConfig struct {
	GitPublicKey  string `yaml:"public_key"`
	GitPrivateKey string `yaml:"private_key"`
}

type DockerConfig struct {
	DockerAPIVersion      string `yaml:"version"`
	DockerLogDriver       string `yaml:"log_driver"`
	DockerNetworkMode     string `yaml:"network_mode"`
	DockerNetworkDisabled bool   `yaml:"network_disabled"`
	DockerCertPath        string `yaml:"cert_path"`
	DockerHub             string `yaml:"hub"`
}
