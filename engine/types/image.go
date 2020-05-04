package types

import "io"

type BuildType int

const (
	BuildFromUnknown BuildType = iota
	BuildFromSCM
	BuildFromContent
	BuildFromExist
)

// Image contain image meta data
type Image struct {
	ID   string
	Tags []string
}

// BuildOptions is options for building image
type BuildOptions struct {
	Name string
	User string
	UID  int
	Tags []string
	// used for BuildContent + BuildImage (BuildFromSCM)
	Builds *Builds
	// used for BuildImage (BuildFromContent)
	Tar io.Reader
	// used for BuildFromExist
	FromExist string `yaml:"from_exist,omitempty"`
}

func (o *BuildOptions) BuildType() BuildType {
	if o.FromExist != "" {
		return BuildFromExist
	}
	if o.Tar != nil {
		return BuildFromContent
	}
	if o.Builds != nil {
		return BuildFromSCM
	}
	return BuildFromUnknown
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
	Security   bool              `yaml:"security,omitempty"`
	Commands   []string          `yaml:"commands,omitempty,flow"`
	Envs       map[string]string `yaml:"envs,omitempty,flow"`
	Args       map[string]string `yaml:"args,omitempty,flow"`
	Labels     map[string]string `yaml:"labels,omitempty,flow"`
	Artifacts  map[string]string `yaml:"artifacts,omitempty,flow"`
	Cache      map[string]string `yaml:"cache,omitempty,flow"`
	StopSignal string            `yaml:"stop_signal,omitempty,flow"`
}
