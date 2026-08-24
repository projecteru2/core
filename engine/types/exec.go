package types

// ExecConfig mirrors the exec subset of docker's api/types Config.
type ExecConfig struct {
	User         string
	Privileged   bool
	Tty          bool
	AttachStdin  bool
	AttachStderr bool
	AttachStdout bool
	Detach       bool
	DetachKeys   string // docker --detach-keys format
	Env          []string
	WorkingDir   string
	Cmd          []string
}
