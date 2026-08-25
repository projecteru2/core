package types

import (
	"encoding/json"
	"strings"
)

// SplitRef separates an image reference from its tag, ignoring a registry port.
func SplitRef(ref string) (name, tag string) {
	colon := strings.LastIndex(ref, ":")
	if colon < 0 || colon < strings.LastIndex(ref, "/") {
		return ref, ""
	}
	return ref[:colon], ref[colon+1:]
}

// IsURL reports whether the image is a plain download url rather than a registry ref.
func IsURL(image string) bool {
	return strings.HasPrefix(image, "http://") || strings.HasPrefix(image, "https://")
}

// ImageDigest renders the digest form core compares; a cloud image url stands for itself.
func ImageDigest(image, digest string) string {
	if IsURL(image) {
		return image
	}
	name, _ := SplitRef(image)
	return name + "@" + digest
}

// ParseDescriptor reads the digest out of an OCI descriptor.
func ParseDescriptor(out string) (string, error) {
	descriptor := struct {
		Digest string `json:"digest"`
	}{}
	if err := json.Unmarshal([]byte(out), &descriptor); err != nil {
		return "", err
	}
	return descriptor.Digest, nil
}

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
