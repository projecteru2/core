package docker

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/log"
	coresource "github.com/projecteru2/core/source"
	coretypes "github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

const (
	fromAsTmpl = "FROM %s as %s"
	commonTmpl = `{{ range $k, $v:= .Args -}}
{{ printf "ARG %s=%q" $k $v }}
{{ end -}}
{{ range $k, $v:= .Envs -}}
{{ printf "ENV %s %q" $k $v }}
{{ end -}}
{{ range $k, $v:= .Labels -}}
{{ printf "LABEL %s=%s" $k $v }}
{{ end -}}
{{- if .Dir}}RUN mkdir -p {{.Dir}}
WORKDIR {{.Dir}}{{ end }}
{{ if .Repo }}ADD {{.Repo}} .{{ end }}
{{ if .StopSignal }}STOPSIGNAL {{.StopSignal}} {{ end }}`
	copyTmpl = "COPY --from=%s %s %s"
	runTmpl  = "RUN %s"
	userTmpl = `RUN echo "{{.User}}::{{.UID}}:{{.UID}}:{{.User}}:/dev/null:/sbin/nologin" >> /etc/passwd && \
echo "{{.User}}:x:{{.UID}}:" >> /etc/group && \
echo "{{.User}}:!::0:::::" >> /etc/shadow
USER {{.User}}
`
)

func (e *Engine) BuildRefs(_ context.Context, opts *enginetypes.BuildRefOptions) []string {
	name := opts.Name
	tags := opts.Tags
	refs := []string{}
	for _, tag := range tags {
		ref := createImageTag(e.config.Docker, name, tag)
		refs = append(refs, ref)
	}
	if len(refs) == 0 {
		refs = append(refs, createImageTag(e.config.Docker, name, utils.DefaultVersion))
	}
	return refs
}

// layout: <buildDir>/<reponame>/<code> next to <buildDir>/Dockerfile
func (e *Engine) BuildContent(ctx context.Context, scm coresource.Source, opts *enginetypes.BuildContentOptions) (string, io.Reader, error) {
	if opts.Builds == nil {
		return "", nil, coretypes.ErrNoBuildsInSpec
	}
	buildDir, err := os.MkdirTemp(os.TempDir(), "corebuild-")
	if err != nil {
		return "", nil, err
	}
	log.WithFunc("engine.docker.BuildContent").Debugf(ctx, "build dir %s", buildDir)
	if err = e.makeDockerFile(ctx, opts, scm, buildDir); err != nil {
		return buildDir, nil, err
	}
	tar, err := CreateTarStream(buildDir)
	return buildDir, tar, err
}

func (e *Engine) makeDockerFile(ctx context.Context, opts *enginetypes.BuildContentOptions, scm coresource.Source, buildDir string) error {
	var preCache map[string]string
	var preStage string
	var buildTmpl []string

	for _, stage := range opts.Stages {
		build, ok := opts.Builds.Builds[stage]
		if !ok {
			log.WithFunc("engine.docker.makeDockerFile").Warnf(ctx, "build stage %s not defined", stage)
			continue
		}

		reponame, err := e.preparedSource(ctx, build, scm, buildDir)
		if err != nil {
			return err
		}
		build.Repo = reponame

		from := fmt.Sprintf(fromAsTmpl, build.Base, stage)

		copys := []string{}
		for src, dst := range preCache {
			copys = append(copys, fmt.Sprintf(copyTmpl, preStage, src, dst))
		}

		commands := []string{}
		for _, command := range build.Commands {
			commands = append(commands, fmt.Sprintf(runTmpl, command))
		}

		mainPart, err := makeMainPart(opts, build, from, commands, copys)
		if err != nil {
			return err
		}
		buildTmpl = append(buildTmpl, mainPart)
		preStage = stage
		preCache = build.Cache
	}

	if opts.User != "" && opts.UID != 0 {
		userPart, err := makeUserPart(opts)
		if err != nil {
			return err
		}
		buildTmpl = append(buildTmpl, userPart)
	}
	dockerfile := strings.Join(buildTmpl, "\n")
	return createDockerfile(dockerfile, buildDir)
}

func (e *Engine) preparedSource(ctx context.Context, build *enginetypes.Build, scm coresource.Source, buildDir string) (string, error) {
	var cloneDir string
	var err error
	reponame := ""
	if build.Repo != "" { //nolint:nestif
		version := build.Version
		if version == "" {
			version = "HEAD"
		}
		reponame, err = utils.GetGitRepoName(build.Repo)
		if err != nil {
			return "", err
		}

		cloneDir = filepath.Join(buildDir, reponame)
		if err := scm.SourceCode(ctx, build.Repo, cloneDir, version, build.Submodule); err != nil {
			return "", err
		}

		if build.Security {
			// Security strips the .git directory from the build context
			if err := scm.Security(cloneDir); err != nil {
				return "", err
			}
		}
	}

	// artifacts replace the cloned tree so no source ships in the image
	if len(build.Artifacts) > 0 {
		artifactsDir := buildDir
		if cloneDir != "" {
			if err := recreateDir(cloneDir); err != nil {
				return "", err
			}
			artifactsDir = cloneDir
		}
		for _, artifact := range build.Artifacts {
			if err := scm.Artifact(ctx, artifact, artifactsDir); err != nil {
				return "", err
			}
		}
	}

	return reponame, nil
}
