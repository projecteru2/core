package source

import "context"

// Source defines SCM funcions
type Source interface {
	// Get source code from repository into path by revision
	SourceCode(ctx context.Context, repository, path, revision string, submodule bool) error
	// Get related artifact by artifact into path
	Artifact(_ context.Context, artifact, path string) error
	// Keep code security
	Security(path string) error
}
