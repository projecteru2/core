package calcium

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	enginetypes "github.com/docker/docker/api/types"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
	log "github.com/sirupsen/logrus"
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

func (c *Calcium) preparedSource(build *types.Build, buildDir string) (string, error) {
	// parse repository name
	// code locates under /:repositoryname
	var cloneDir string
	var err error
	reponame := ""
	if build.Repo != "" {
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
		if err := c.source.SourceCode(build.Repo, cloneDir, version, build.Submodule); err != nil {
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

func (c *Calcium) makeDockerFile(opts *types.BuildOptions, buildDir string) error {
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

// BuildImage will build image for repository
// since we wanna set UID for the user inside container, we have to know the uid parameter
//
// build directory is like:
//
//    buildDir ├─ :appname ├─ code
//             ├─ Dockerfile
func (c *Calcium) BuildImage(ctx context.Context, opts *types.BuildOptions) (chan *types.BuildImageMessage, error) {
	ch := make(chan *types.BuildImageMessage)

	// get pod from config
	buildPodname := c.config.Docker.BuildPod
	if buildPodname == "" {
		return ch, types.ErrNoBuildPod
	}

	// get node by scheduler
	nodes, err := c.ListPodNodes(ctx, buildPodname, false)
	if err != nil {
		return ch, err
	}
	if len(nodes) == 0 {
		return ch, types.ErrInsufficientNodes
	}
	//TODO 这里需要考虑用 mem，CPU 不限制 mem
	node := c.scheduler.MaxCPUIdleNode(nodes)
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
	tags := []string{}
	for i := range opts.Tags {
		if opts.Tags[i] != "" {
			tag := createImageTag(c.config.Docker, opts.Name, opts.Tags[i])
			tags = append(tags, tag)
		}
	}
	// use latest
	if len(tags) == 0 {
		tags = append(tags, createImageTag(c.config.Docker, opts.Name, utils.DefaultVersion))
	}

	// create tar stream for Build API
	buildContext, err := createTarStream(buildDir)
	if err != nil {
		return ch, err
	}

	// must be put here because of that `defer os.RemoveAll(buildDir)`
	buildOptions := enginetypes.ImageBuildOptions{
		Tags:           tags,
		SuppressOutput: false,
		NoCache:        true,
		Remove:         true,
		ForceRemove:    true,
		PullParent:     true,
	}

	log.Infof("[BuildImage] Building image %v:%v", buildPodname, node.Name)
	resp, err := node.Engine.ImageBuild(ctx, buildContext, buildOptions)
	if err != nil {
		return ch, err
	}

	go func() {
		defer resp.Body.Close()
		defer close(ch)
		decoder := json.NewDecoder(resp.Body)
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

		// push and clean
		for i := range tags {
			tag := tags[i]
			log.Infof("[BuildImage] Push image %s", tag)
			encodedAuth, err := makeEncodedAuthConfigFromRemote(c.config.Docker.AuthConfigs, tag)
			if err != nil {
				ch <- makeErrorBuildImageMessage(err)
				continue
			}
			pushOptions := enginetypes.ImagePushOptions{RegistryAuth: encodedAuth}

			rc, err := node.Engine.ImagePush(ctx, tag, pushOptions)
			if err != nil {
				ch <- makeErrorBuildImageMessage(err)
				continue
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
					break
				}
				ch <- message
			}

			// 无论如何都删掉build机器的
			// 事实上他不会跟cached pod一样
			// 一样就砍死
			go func(tag string) {
				//CONTEXT 这里的不应该受到 client 的影响
				ctx := context.Background()
				_, err := node.Engine.ImageRemove(ctx, tag, enginetypes.ImageRemoveOptions{
					Force:         false,
					PruneChildren: true,
				})
				if err != nil {
					log.Errorf("[BuildImage] Remove image error: %s", err)
				}
				r, err := node.Engine.BuildCachePrune(ctx, enginetypes.BuildCachePruneOptions{All: true})
				if err != nil {
					log.Errorf("[BuildImage] Remove build image cache error: %s", err)
				}
				log.Debugf("[BuildImage] Clean cached image and release space %d", r.SpaceReclaimed)
			}(tag)

			ch <- &types.BuildImageMessage{Stream: fmt.Sprintf("finished %s\n", tag), Status: "finished", Progress: tag}
		}
	}()

	return ch, nil
}
