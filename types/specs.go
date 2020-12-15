package types

import (
	"strings"
)

// Hook define hooks
type Hook struct {
	AfterStart []string `yaml:"after_start,omitempty"`
	BeforeStop []string `yaml:"before_stop,omitempty"`
	Force      bool     `yaml:"force,omitempty"`
}

// HealthCheck define healthcheck
type HealthCheck struct {
	TCPPorts []string `yaml:"tcp_ports,omitempty,flow"`
	HTTPPort string   `yaml:"http_port"`
	HTTPURL  string   `yaml:"url,omitempty"`
	HTTPCode int      `yaml:"code,omitempty"`
}

// Entrypoint is a single entrypoint
type Entrypoint struct {
	Name          string            `yaml:"name,omitempty"`
	Command       string            `yaml:"cmd,omitempty"`
	Privileged    bool              `yaml:"privileged,omitempty"`
	Dir           string            `yaml:"dir,omitempty"`
	Log           *LogConfig        `yaml:"log,omitempty"`
	Publish       []string          `yaml:"publish,omitempty,flow"`
	HealthCheck   *HealthCheck      `yaml:"healthcheck,omitempty,flow"`
	Hook          *Hook             `yaml:"hook,omitempty,flow"`
	RestartPolicy string            `yaml:"restart,omitempty"`
	Sysctls       map[string]string `yaml:"sysctls,omitempty,flow"`
}

// Validate checks entrypoint's name
func (e *Entrypoint) Validate() error {
	if e.Name == "" {
		return ErrEmptyEntrypointName
	}
	if strings.Contains(e.Name, "_") {
		return ErrUnderlineInEntrypointName
	}
	return nil
}

// Bind define a single bind
type Bind struct {
	InWorkloadPath string `yaml:"bind,omitempty"`
	ReadOnly       bool   `yaml:"ro,omitempty"`
}
