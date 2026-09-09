package types

// ExecConfig mirrors the exec subset of docker's api/types Config.
type ExecConfig struct {
	User        string
	Privileged  bool
	Tty         bool
	AttachStdin bool
	Env         []string
	WorkingDir  string
	Cmd         []string
}
