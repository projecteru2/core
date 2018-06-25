package types

// correspond to app.yaml in repository
type Builds struct {
	Stages []string          `yaml:"stages,omitempty,flow"`
	Builds map[string]*Build `yaml:"builds,omitempty,flow"`
}

type Build struct {
	Base      string            `yaml:"base,omitempty"`
	Repo      string            `yaml:"repo,omitempty"`
	Version   string            `yaml:"version,omitempty"`
	Dir       string            `yaml:"dir,omitempty"`
	Submodule bool              `yaml:"submodule,omitempty"`
	Commands  []string          `yaml:"commands,omitempty,flow"`
	Envs      map[string]string `yaml:"envs,omitempty,flow"`
	Args      map[string]string `yaml:"args,omitempty,flow"`
	Labels    map[string]string `yaml:"labels,omitempty,flow"`
	Artifacts map[string]string `yaml:"artifacts,omitempty,flow"`
	Cache     map[string]string `yaml:"cache,omitempty,flow"`
}

type Hook struct {
	AfterStart []string `yaml:"after_start,omitempty"`
	BeforeStop []string `yaml:"before_stop,omitempty"`
	Force      bool     `yaml:"force,omitempty"`
}

type HealthCheck struct {
	TCPPorts []string `yaml:"tcp_ports,omitempty,flow"`
	HTTPPort string   `yaml:"http_port"`
	HTTPURL  string   `yaml:"url,omitempty"`
	HTTPCode int      `yaml:"code,omitempty"`
}

// single entrypoint
type Entrypoint struct {
	Name          string       `yaml:"name,omitempty"`
	Command       string       `yaml:"cmd,omitempty"`
	Privileged    bool         `yaml:"privileged,omitempty"`
	Dir           string       `yaml:"dir,omitempty"`
	LogConfig     string       `yaml:"log_config,omitempty"`
	Publish       []string     `yaml:"publish,omitempty,flow"`
	HealthCheck   *HealthCheck `yaml:"healthcheck,omitempty,flow"`
	Hook          *Hook        `yaml:"hook,omitempty,flow"`
	RestartPolicy string       `yaml:"restart,omitempty"`
}

// single bind
type Bind struct {
	InContainerPath string `yaml:"bind,omitempty"`
	ReadOnly        bool   `yaml:"ro,omitempty"`
}
