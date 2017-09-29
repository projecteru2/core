package types

import (
	"strings"
)

// correspond to app.yaml in repository
type Builds struct {
	Stages []string          `yaml:"stages,omitempty,flow"`
	Builds map[string]*Build `yaml:"builds,omitempty,flow"`
}

type Build struct {
	Base      string            `yaml:"base,omitempty"`
	Repo      string            `yaml:"repo,omitempty"`
	Source    bool              `yaml:"source,omitempty"`
	Version   string            `yaml:"version,omitempty"`
	Commands  []string          `yaml:"commands,omitempty,flow"`
	Artifacts map[string]string `yaml:"artifacts,omitempty,flow"`
}

type Hook struct {
	AfterStart string `yaml:"after_start,omitempty"`
	BeforeStop string `yaml:"before_stop,omitempty"`
}

type HealthCheck struct {
	Port int    `yaml:"healthcheck_port,omitempty,flow"`
	URL  string `yaml:"healthcheck_url,omitempty"`
	Code int    `yaml:"healthcheck_expected_code,omitempty"`
}

// single entrypoint
type Entrypoint struct {
	Name          string       `yaml:"name,omitempty"`
	Command       string       `yaml:"cmd,omitempty"`
	Privileged    string       `yaml:"privileged,omitempty"`
	WorkingDir    string       `yaml:"working_dir,omitempty"`
	LogConfig     string       `yaml:"log_config,omitempty"`
	Ports         []Port       `yaml:"ports,omitempty,flow"`
	HealthCheck   *HealthCheck `yaml:"healthcheck,omitempty,flow"`
	Hook          *Hook        `yaml:"hook,omitempty,flow"`
	RestartPolicy string       `yaml:"restart,omitempty"`
	ExtraHosts    []string     `yaml:"hosts,omitempty,flow"`
}

// single bind
type Bind struct {
	InContainerPath string `yaml:"bind,omitempty"`
	ReadOnly        bool   `yaml:"ro,omitempty"`
}

type Port string

// port is in form of 5000/tcp
func (p Port) Port() string {
	return strings.Split(string(p), "/")[0]
}

// default protocol is tcp
func (p Port) Proto() string {
	parts := strings.Split(string(p), "/")
	if len(parts) == 1 {
		return "tcp"
	}
	return parts[1]
}
