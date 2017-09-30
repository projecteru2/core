package types

//BuildOptions is options for building image
type BuildOptions struct {
	Name   string
	User   string
	UID    int
	Tag    string
	Builds *Builds
}

//DeployOptions is options for deploying
type DeployOptions struct {
	Name        string            // Name of application
	Entrypoint  *Entrypoint       // entrypoint
	Podname     string            // Name of pod to deploy
	Nodename    string            // Specific nodes to deploy, if given, must belong to pod
	Image       string            // Name of image to deploy
	ExtraArgs   string            // Extra arguments to append to command
	CPUQuota    float64           // How many cores needed, e.g. 1.5
	Memory      int64             // Memory for container, in bytes
	Count       int               // How many containers needed, e.g. 4
	Env         []string          // Env for container
	DNS         []string          // DNS for container
	Volumes     []string          // Volumes for container
	Networks    map[string]string // Network names and specified IPs
	NetworkMode string            // Network mode
	User        string            // User for container
	Debug       bool              // debug mode, use syslog as log driver
	OpenStdin   bool              // OpenStdin for container
	Meta        map[string]string // Meta for containers
}

//RunAndWaitOptions is options for running and waiting
type RunAndWaitOptions struct {
	DeployOptions
	Timeout int
	Cmd     string
}
