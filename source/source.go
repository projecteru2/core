package source

type Source interface {
	// Get source code from repository into path by revision
	SourceCode(repository, path, revision string) error
	// Get related artifact by artifact into path
	Artifact(artifact, path string) error
}
