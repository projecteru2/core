package types

// DeployOptions is options for deploying
type DeployOptions struct {
	Name         string            // Name of application
	Entrypoint   *Entrypoint       // entrypoint
	Podname      string            // Name of pod to deploy
	Nodename     string            // Specific nodes to deploy, if given, must belong to pod
	Image        string            // Name of image to deploy
	ExtraArgs    string            // Extra arguments to append to command
	CPUQuota     float64           // How many cores needed, e.g. 1.5
	Memory       int64             // Memory for container, in bytes
	Count        int               // How many containers needed, e.g. 4
	Env          []string          // Env for container
	DNS          []string          // DNS for container
	ExtraHosts   []string          // Extra hosts for container
	Volumes      []string          // Volumes for container
	Networks     map[string]string // Network names and specified IPs
	NetworkMode  string            // Network mode
	User         string            // User for container
	Debug        bool              // debug mode, use syslog as log driver
	OpenStdin    bool              // OpenStdin for container
	Labels       map[string]string // Labels for containers
	NodeLabels   map[string]string // NodeLabels for filter node
	DeployMethod string            // Deploy method
	Data         map[string]string // For additional file data
	SoftLimit    bool              // Softlimit memory
	NodesLimit   int               // Limit nodes count
	ProcessIdent string            // ProcessIdent ident this deploy
}

// RunAndWaitOptions is options for running and waiting
type RunAndWaitOptions struct {
	DeployOptions
	Timeout int
	Cmd     string
}

// CopyOptions for multiple container files copy
type CopyOptions struct {
	Targets map[string][]string
}

// ListContainersOptions for list containers
type ListContainersOptions struct {
	Appname    string
	Entrypoint string
	Nodename   string
}

// ReplaceOptions for replace container
type ReplaceOptions struct {
	DeployOptions
	Force          bool
	FilterLabels   map[string]string
	Copy           map[string]string
	IDs            []string
	NetworkInherit bool
}
