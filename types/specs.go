package types

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v2"
)

// correspond to app.yaml in repository
type Specs struct {
	Appname     string                `yaml:"appname,omitempty"`
	Entrypoints map[string]Entrypoint `yaml:"entrypoints,omitempty,flow"`
	Build       []string              `yaml:"build,omitempty,flow"`
	Volumes     []string              `yaml:"volumes,omitempty,flow"`
	Binds       map[string]Bind       `yaml:"binds,omitempty,flow"`
	Meta        map[string]string     `yaml:"meta,omitempty,flow"`
	Base        string                `yaml:"base"`
	MountPaths  []string              `yaml:"mount_paths,omitempty,flow"`
	DNS         []string              `yaml:"dns,omitempty,flow"`
}

// single entrypoint
type Entrypoint struct {
	Command                 string   `yaml:"cmd,omitempty"`
	AfterStart              string   `yaml:"after_start,omitempty"`
	BeforeStop              string   `yaml:"before_stop,omitempty"`
	Ports                   []Port   `yaml:"ports,omitempty,flow"`
	Exposes                 []Expose `yaml:"exposes,omitempty,flow"`
	NetworkMode             string   `yaml:"network_mode,omitempty"`
	RestartPolicy           string   `yaml:"restart,omitempty"`
	HealthCheckUrl          string   `yaml:"healthcheck_url,omitempty"`
	HealthCheckExpectedCode int      `yaml:"healthcheck_expected_code,omitempty"`
	ExtraHosts              []string `yaml:"hosts,omitempty,flow"`
	PermDir                 bool     `yaml:"permdir,omitempty"`
	Privileged              string   `yaml:"privileged,omitempty"`
	LogConfig               string   `yaml:"log_config,omitempty"`
	WorkingDir              string   `yaml:"working_dir,omitempty"`
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

type Expose string

// suppose expose is like 80/tcp:46656/tcp
func (e Expose) ContainerPort() Port {
	ports := strings.Split(string(e), ":")
	return Port(ports[0])
}

func (e Expose) HostPort() Port {
	ports := strings.Split(string(e), ":")
	return Port(ports[1])
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
