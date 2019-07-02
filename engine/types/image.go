package types

import "io"

// Image contain image meta data
type Image struct {
	ID   string
	Tags []string
}

// BuildOptions is options for building image
type BuildOptions struct {
	Name   string
	User   string
	UID    int
	Tags   []string
	Builds *Builds
	Tar    io.Reader
}

// Builds define builds
type Builds struct {
	Stages []string          `yaml:"stages,omitempty,flow"`
	Builds map[string]*Build `yaml:"builds,omitempty,flow"`
}

// Build define build
type Build struct {
	Base       string            `yaml:"base,omitempty"`
	Repo       string            `yaml:"repo,omitempty"`
	Version    string            `yaml:"version,omitempty"`
	Dir        string            `yaml:"dir,omitempty"`
	Submodule  bool              `yaml:"submodule,omitempty"`
	Commands   []string          `yaml:"commands,omitempty,flow"`
	Envs       map[string]string `yaml:"envs,omitempty,flow"`
	Args       map[string]string `yaml:"args,omitempty,flow"`
	Labels     map[string]string `yaml:"labels,omitempty,flow"`
	Artifacts  map[string]string `yaml:"artifacts,omitempty,flow"`
	Cache      map[string]string `yaml:"cache,omitempty,flow"`
	StopSignal string            `yaml:"stop_signal,omitempty,flow"`
}
