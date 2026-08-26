package containerd

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/moby/go-archive"
	"github.com/moby/go-archive/compression"

	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/log"
	coresource "github.com/projecteru2/core/source"
	"github.com/projecteru2/core/utils"
)

const (
	dockerfileName = "Dockerfile"

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

func makeDockerfile(ctx context.Context, opts *enginetypes.BuildContentOptions, scm coresource.Source, buildDir string) error {
	var preCache map[string]string
	var preStage string
	var buildTmpl []string

	for _, stage := range opts.Stages {
		build, ok := opts.Builds.Builds[stage]
		if !ok {
			log.WithFunc("engine.containerd.makeDockerfile").Warnf(ctx, "build stage %s not defined", stage)
			continue
		}

		reponame, err := preparedSource(ctx, build, scm, buildDir)
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

		mainPart, err := makeMainPart(build, from, commands, copys)
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
	return writeDockerfile(strings.Join(buildTmpl, "\n"), buildDir)
}

func preparedSource(ctx context.Context, build *enginetypes.Build, scm coresource.Source, buildDir string) (string, error) {
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

func makeMainPart(build *enginetypes.Build, from string, commands, copys []string) (string, error) {
	common, err := renderTemplate("common", commonTmpl, build)
	if err != nil {
		return "", err
	}
	buildTmpl := []string{from, common}
	buildTmpl = append(buildTmpl, copys...)
	buildTmpl = append(buildTmpl, commands...)
	return strings.Join(buildTmpl, "\n"), nil
}

func makeUserPart(opts *enginetypes.BuildContentOptions) (string, error) {
	return renderTemplate("user", userTmpl, opts)
}

func renderTemplate(name, body string, data any) (string, error) {
	tmpl := template.Must(template.New(name).Parse(body))
	out := bytes.Buffer{}
	if err := tmpl.Execute(&out, data); err != nil {
		return "", err
	}
	return out.String(), nil
}

func recreateDir(path string) error {
	if err := os.RemoveAll(path); err != nil {
		return err
	}
	return os.MkdirAll(path, 0o750)
}

func writeDockerfile(dockerfile, buildDir string) (err error) {
	f, err := os.Create(filepath.Clean(filepath.Join(buildDir, dockerfileName)))
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := f.Close(); err == nil {
			err = closeErr
		}
	}()
	_, err = f.WriteString(dockerfile)
	return err
}

func createTarStream(path string) (io.ReadCloser, error) {
	return archive.TarWithOptions(path, &archive.TarOptions{
		ExcludePatterns: []string{},
		IncludeFiles:    []string{"."},
		Compression:     compression.None,
		NoLchown:        true,
	})
}

func unpackContext(input io.Reader, dir string) error {
	return archive.Untar(input, dir, &archive.TarOptions{NoLchown: true})
}
