package types

// Image is an image's ID and tags.
type Image struct {
	ID   string
	Tags []string
}

type BuildContentOptions struct {
	User string
	UID  int
	*Builds
}

type BuildRefOptions struct {
	Name string
	Tags []string
	User string
}

type Builds struct {
	Stages []string          `yaml:"stages,omitempty,flow"`
	Builds map[string]*Build `yaml:"builds,omitempty,flow"`
}

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
