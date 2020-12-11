package docker

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/projecteru2/core/engine/types"
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
	// TODO consider work dir privilege
	// Add user manually
	userTmpl = `RUN echo "{{.User}}::{{.UID}}:{{.UID}}:{{.User}}:/dev/null:/sbin/nologin" >> /etc/passwd && \
echo "{{.User}}:x:{{.UID}}:" >> /etc/group && \
echo "{{.User}}:!::0:::::" >> /etc/shadow
USER {{.User}}
`
)

// BuildRefs output refs
func (e *Engine) BuildRefs(ctx context.Context, name string, tags []string) []string {
	refs := []string{}
	for _, tag := range tags {
		ref := createImageTag(e.config.Docker, name, tag)
		refs = append(refs, ref)
	}
	// use latest
	if len(refs) == 0 {
		refs = append(refs, createImageTag(e.config.Docker, name, utils.DefaultVersion))
	}
	return refs
}

// BuildContent generate build content
// since we wanna set UID for the user inside workload, we have to know the uid parameter
//
// build directory is like:
//
//    buildDir ├─ :appname ├─ code
//             ├─ Dockerfile
func (e *Engine) BuildContent(ctx context.Context, scm coresource.Source, opts *enginetypes.BuildContentOptions) (string, io.Reader, error) {
	if opts.Builds == nil {
		return "", nil, coretypes.ErrNoBuildsInSpec
	}
	// make build dir
	buildDir, err := ioutil.TempDir(os.TempDir(), "corebuild-")
	if err != nil {
		return "", nil, err
	}
	log.Debugf("[BuildContent] Build dir %s", buildDir)
	// create dockerfile
	if err := e.makeDockerFile(ctx, opts, scm, buildDir); err != nil {
		return buildDir, nil, err
	}
	// create stream for Build API
	tar, err := CreateTarStream(buildDir)
	return buildDir, tar, err
}

func (e *Engine) makeDockerFile(ctx context.Context, opts *types.BuildContentOptions, scm coresource.Source, buildDir string) error {
	var preCache map[string]string
	var preStage string
	var buildTmpl []string

	for _, stage := range opts.Builds.Stages {
		build, ok := opts.Builds.Builds[stage]
		if !ok {
			log.Warnf("[makeDockerFile] Builds stage %s not defined", stage)
			continue
		}

		// get source or artifacts
		reponame, err := e.preparedSource(ctx, build, scm, buildDir)
		if err != nil {
			return err
		}
		build.Repo = reponame

		// get header
		from := fmt.Sprintf(fromAsTmpl, build.Base, stage)

		// get multiple stags
		copys := []string{}
		for src, dst := range preCache {
			copys = append(copys, fmt.Sprintf(copyTmpl, preStage, src, dst))
		}

		// get commands
		commands := []string{}
		for _, command := range build.Commands {
			commands = append(commands, fmt.Sprintf(runTmpl, command))
		}

		// decide add source or not
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

func (e *Engine) preparedSource(ctx context.Context, build *types.Build, scm coresource.Source, buildDir string) (string, error) {
	// parse repository name
	// code locates under /:repositoryname
	var cloneDir string
	var err error
	reponame := ""
	if build.Repo != "" { // nolint
		version := build.Version
		if version == "" {
			version = "HEAD"
		}
		reponame, err = utils.GetGitRepoName(build.Repo)
		if err != nil {
			return "", err
		}

		// clone code into cloneDir
		// which is under buildDir and named as repository name
		cloneDir = filepath.Join(buildDir, reponame)
		if err := scm.SourceCode(ctx, build.Repo, cloneDir, version, build.Submodule); err != nil {
			return "", err
		}

		if build.Security {
			// ensure source code is safe
			// we don't want any history files to be retrieved
			if err := scm.Security(cloneDir); err != nil {
				return "", err
			}
		}
	}

	// if artifact download url is provided, remove all source code to
	// improve security
	if len(build.Artifacts) > 0 {
		artifactsDir := buildDir
		if cloneDir != "" {
			os.RemoveAll(cloneDir)
			if err := os.MkdirAll(cloneDir, os.ModeDir); err != nil {
				return "", err
			}
			artifactsDir = cloneDir
		}
		for _, artifact := range build.Artifacts {
			if err := scm.Artifact(artifact, artifactsDir); err != nil {
				return "", err
			}
		}
	}

	return reponame, nil
}
