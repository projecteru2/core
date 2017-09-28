package types

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v2"
)

// correspond to app.yaml in repository
type Specs struct {
	Appname      string                `yaml:"appname,omitempty"`
	Entrypoints  map[string]Entrypoint `yaml:"entrypoints,omitempty,flow"`
	Build        []string              `yaml:"build,omitempty,flow"`
	ComplexBuild ComplexBuild          `yaml:"complex_build,omitempty,flow"`
	Volumes      []string              `yaml:"volumes,omitempty,flow"`
	Meta         map[string]string     `yaml:"meta,omitempty,flow"`
	Base         string                `yaml:"base"`
	DNS          []string              `yaml:"dns,omitempty,flow"`
}

type ComplexBuild struct {
	Stages []string         `yaml:"stages,omitempty,flow"`
	Builds map[string]Build `yaml:"builds,omitempty,flow"`
}

type Build struct {
	Base      string            `yaml:"base,omitempty"`
	Source    bool              `yaml:"source,omitempty"`
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
	Name          string      `yaml:"name,omitempty"`
	Command       string      `yaml:"cmd,omitempty"`
	Privileged    string      `yaml:"privileged,omitempty"`
	WorkingDir    string      `yaml:"working_dir,omitempty"`
	LogConfig     string      `yaml:"log_config,omitempty"`
	Ports         []Port      `yaml:"ports,omitempty,flow"`
	HealthCheck   HealthCheck `yaml:"healthcheck,omitempty,flow"`
	Hook          Hook        `yaml:"hook,omitempty,flow"`
	RestartPolicy string      `yaml:"restart,omitempty"`
	ExtraHosts    []string    `yaml:"hosts,omitempty,flow"`
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

// load Specs from content
func LoadSpecs(content string) (Specs, error) {
	specs := Specs{}
	err := yaml.Unmarshal([]byte(content), &specs)
	if err != nil {
		return specs, err
	}

	err = verify(specs)
	return specs, err
}

// basic verification
// TODO need more ports verification
func verify(a Specs) error {
	if a.Appname == "" {
		return fmt.Errorf("No appname specified")
	}
	if len(a.Entrypoints) == 0 {
		return fmt.Errorf("No entrypoints specified")
	}

	for name, _ := range a.Entrypoints {
		if strings.Contains(name, "_") {
			return fmt.Errorf("Sorry but we do not support `_` in entrypoint 눈_눈")
		}
	}
	return nil
}
