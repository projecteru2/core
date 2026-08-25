package source

import "context"

// Source defines the SCM operations a build needs.
type Source interface {
	// SourceCode clones repository at revision into path.
	SourceCode(ctx context.Context, repository, path, revision string, submodule bool) error
	// Artifact downloads artifact into path and unpacks it.
	Artifact(ctx context.Context, artifact, path string) error
	// Security removes VCS metadata from path.
	Security(path string) error
}
