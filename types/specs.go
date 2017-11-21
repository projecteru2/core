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
	Version   string            `yaml:"version,omitempty"`
	Dir       string            `yaml:"dir,omitempty"`
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
	Ports []Port `yaml:"ports,omitempty,flow"`
	URL   string `yaml:"url,omitempty"`
	Code  int    `yaml:"expected_code,omitempty"`
}

// single entrypoint
type Entrypoint struct {
	Name          string       `yaml:"name,omitempty"`
	Command       string       `yaml:"cmd,omitempty"`
	Privileged    bool         `yaml:"privileged,omitempty"`
	Dir           string       `yaml:"dir,omitempty"`
	LogConfig     string       `yaml:"log_config,omitempty"`
	Publish       []Port       `yaml:"publish,omitempty,flow"`
	HealthCheck   *HealthCheck `yaml:"healthcheck,omitempty,flow"`
	Hook          *Hook        `yaml:"hook,omitempty,flow"`
	RestartPolicy string       `yaml:"restart,omitempty"`
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
