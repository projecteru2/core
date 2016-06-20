package types

type Config struct {
	Bind         string   `yaml:"bind"`
	AgentPort    string   `yaml:"agent_port"`
	PermDir      string   `yaml:"permdir"`
	EtcdMachines []string `yaml:"etcd"`

	Git    GitConfig    `yaml:"git"`
	Docker DockerConfig `yaml:"docker"`
}

type GitConfig struct {
	PublicKey  string `yaml:"public_key"`
	PrivateKey string `yaml:"private_key"`
}

type DockerConfig struct {
	APIVersion  string `yaml:"version"`
	LogDriver   string `yaml:"log_driver"`
	NetworkMode string `yaml:"network_mode"`
	CertPath    string `yaml:"cert_path"`
	Hub         string `yaml:"hub"`
}
