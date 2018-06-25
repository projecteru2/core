package source

// Source defines SCM funcions
type Source interface {
	// Get source code from repository into path by revision
	SourceCode(repository, path, revision string, submodule bool) error
	// Get related artifact by artifact into path
	Artifact(artifact, path string) error
	// Keep code security
	Security(path string) error
}
