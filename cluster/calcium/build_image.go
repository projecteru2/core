package calcium

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	log "github.com/Sirupsen/logrus"
	enginetypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/pkg/archive"
	"github.com/projecteru2/core/types"
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
{{ if .Repo }}ADD {{.Repo}} .{{ end }}`
	copyTmpl = "COPY --from=%s %s %s"
	runTmpl  = "RUN %s"
	//TODO consider work dir privilege
	//Add user manually
	userTmpl = `RUN echo "{{.User}}::{{.UID}}:{{.UID}}:{{.User}}:/dev/null:/sbin/nologin" >> /etc/passwd && \
echo "{{.User}}:x:{{.UID}}:" >> /etc/group && \
echo "{{.User}}:!::0:::::" >> /etc/shadow
USER {{.User}}
`
)

func (c *calcium) preparedSource(build *types.Build, buildDir string) (string, error) {
	// parse repository name
	// code locates under /:repositoryname
	var cloneDir string
	var err error
	reponame := build.Repo
	if build.Repo != "" && build.Version != "" {
		reponame, err = utils.GetGitRepoName(build.Repo)
		if err != nil {
			return "", err
		}

		// clone code into cloneDir
		// which is under buildDir and named as repository name
		cloneDir = filepath.Join(buildDir, reponame)
		if err := c.source.SourceCode(build.Repo, cloneDir, build.Version); err != nil {
			return "", err
		}

		// ensure source code is safe
		// we don't want any history files to be retrieved
		if err := c.source.Security(cloneDir); err != nil {
			return "", err
		}
	}

	// if artifact download url is provided, remove all source code to
	// improve security
	if len(build.Artifacts) > 0 {
		artifactsDir := buildDir
		if cloneDir != "" {
			os.RemoveAll(cloneDir)
			os.MkdirAll(cloneDir, os.ModeDir)
			artifactsDir = cloneDir
		}
		for _, artifact := range build.Artifacts {
			if err := c.source.Artifact(artifact, artifactsDir); err != nil {
				return "", err
			}
		}
	}

	return reponame, nil
}

// BuildImage will build image for repository
// since we wanna set UID for the user inside container, we have to know the uid parameter
//
// build directory is like:
//
//    buildDir ├─ :appname ├─ code
//             ├─ Dockerfile
func (c *calcium) BuildImage(ctx context.Context, opts *types.BuildOptions) (chan *types.BuildImageMessage, error) {
	ch := make(chan *types.BuildImageMessage)

	// get pod from config
	buildPodname := c.config.Docker.BuildPod
	if buildPodname == "" {
		return ch, fmt.Errorf("No build pod set in config")
	}

	// get node by scheduler
	nodes, err := c.ListPodNodes(buildPodname, false)
	if err != nil {
		return ch, err
	}
	if len(nodes) == 0 {
		return ch, errors.New("No node to build")
	}
	node := c.scheduler.MaxIdleNode(nodes)

	// make build dir
	buildDir, err := ioutil.TempDir(os.TempDir(), "corebuild-")
	if err != nil {
		return ch, err
	}
	defer os.RemoveAll(buildDir)

	// create dockerfile
	if err := c.makeDockerFile(opts, buildDir); err != nil {
		return ch, err
	}

	// tag of image, later this will be used to push image to hub
	tag := createImageTag(c.config.Docker, opts.Name, opts.Tag)

	// create tar stream for Build API
	buildContext, err := createTarStream(buildDir)
	if err != nil {
		return ch, err
	}

	// must be put here because of that `defer os.RemoveAll(buildDir)`
	buildOptions := enginetypes.ImageBuildOptions{
		Tags:           []string{tag},
		SuppressOutput: false,
		NoCache:        true,
		Remove:         true,
		ForceRemove:    true,
		PullParent:     true,
	}

	log.Infof("[BuildImage] Building image %v at %v:%v", tag, buildPodname, node.Name)
	resp, err := node.Engine.ImageBuild(ctx, buildContext, buildOptions)
	if err != nil {
		return ch, err
	}

	go func() {
		defer resp.Body.Close()
		defer close(ch)
		decoder := json.NewDecoder(resp.Body)
		var err error
		var lastMessage *types.BuildImageMessage
		for {
			message := &types.BuildImageMessage{}
			err := decoder.Decode(message)
			if err != nil {
				if err == io.EOF {
					break
				}
				if err == context.Canceled || err == context.DeadlineExceeded {
					lastMessage.ErrorDetail.Code = -1
					lastMessage.Error = err.Error()
					break
				}
				malformed := []byte{}
				_, _ = decoder.Buffered().Read(malformed)
				log.Errorf("[BuildImage] Decode build image message failed %v, buffered: %v", err, malformed)
				return
			}
			ch <- message
			lastMessage = message
		}

		if lastMessage.ErrorDetail.Code != 0 {
			log.Errorf("[BuildImage] Build image failed %v", lastMessage.ErrorDetail.Message)
			return
		}
		encodedAuth, err := c.MakeEncodedAuthConfigFromRemote(tag)
		if err != nil {
			ch <- makeErrorBuildImageMessage(err)
			return
		}
		pushOptions := enginetypes.ImagePushOptions{RegistryAuth: encodedAuth}
		rc, err := node.Engine.ImagePush(ctx, tag, pushOptions)
		if err != nil {
			ch <- makeErrorBuildImageMessage(err)
			return
		}

		defer rc.Close()
		decoder2 := json.NewDecoder(rc)
		for {
			message := &types.BuildImageMessage{}
			err := decoder2.Decode(message)
			if err != nil {
				if err == io.EOF {
					break
				}
				malformed := []byte{}
				_, _ = decoder2.Buffered().Read(malformed)
				log.Errorf("[BuildImage] Decode push image message failed %v, buffered: %v", err, malformed)
				return
			}
			ch <- message
		}

		// 无论如何都删掉build机器的
		// 事实上他不会跟cached pod一样
		// 一样就砍死
		go func() {
			_, err := node.Engine.ImageRemove(context.Background(), tag, enginetypes.ImageRemoveOptions{
				Force:         false,
				PruneChildren: true,
			})
			if err != nil {
				log.Errorf("[BuildImage] Remove image error: %s", err)
			}
		}()

		ch <- &types.BuildImageMessage{Stream: fmt.Sprintf("finished %s\n", tag), Status: "finished", Progress: tag}
	}()

	return ch, nil
}

func makeErrorBuildImageMessage(err error) *types.BuildImageMessage {
	return &types.BuildImageMessage{Error: err.Error()}
}

func createTarStream(path string) (io.ReadCloser, error) {
	tarOpts := &archive.TarOptions{
		ExcludePatterns: []string{},
		IncludeFiles:    []string{"."},
		Compression:     archive.Uncompressed,
		NoLchown:        true,
	}
	return archive.TarWithOptions(path, tarOpts)
}

func makeCommonPart(build *types.Build) (string, error) {
	tmpl := template.Must(template.New("common").Parse(commonTmpl))
	out := bytes.Buffer{}
	if err := tmpl.Execute(&out, build); err != nil {
		return "", err
	}
	return out.String(), nil
}

func makeUserPart(opts *types.BuildOptions) (string, error) {
	tmpl := template.Must(template.New("user").Parse(userTmpl))
	out := bytes.Buffer{}
	if err := tmpl.Execute(&out, opts); err != nil {
		return "", err
	}
	return out.String(), nil
}

func makeMainPart(opts *types.BuildOptions, build *types.Build, from string, commands, copys []string) (string, error) {
	var buildTmpl []string
	common, err := makeCommonPart(build)
	if err != nil {
		return "", err
	}
	buildTmpl = append(buildTmpl, from, common)
	if len(copys) > 0 {
		buildTmpl = append(buildTmpl, copys...)
	}
	if len(commands) > 0 {
		buildTmpl = append(buildTmpl, commands...)
	}
	return strings.Join(buildTmpl, "\n"), nil
}

func (c *calcium) makeDockerFile(opts *types.BuildOptions, buildDir string) error {
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
		reponame, err := c.preparedSource(build, buildDir)
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

// Dockerfile
func createDockerfile(dockerfile, buildDir string) error {
	f, err := os.Create(filepath.Join(buildDir, "Dockerfile"))
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(dockerfile)
	return err
}

// Image tag
// 格式严格按照 Hub/HubPrefix/appname:version 来
func createImageTag(config types.DockerConfig, appname, version string) string {
	prefix := strings.Trim(config.Namespace, "/")
	if prefix == "" {
		return fmt.Sprintf("%s/%s:%s", config.Hub, appname, version)
	}
	return fmt.Sprintf("%s/%s/%s:%s", config.Hub, prefix, appname, version)
}
