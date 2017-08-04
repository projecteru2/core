package calcium

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	log "github.com/Sirupsen/logrus"
	enginetypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/pkg/archive"
	"gitlab.ricebook.net/platform/core/types"
	"gitlab.ricebook.net/platform/core/utils"
	"gopkg.in/yaml.v2"
)

// FIXME in alpine, useradd rename as adduser
const (
	FROM   = "FROM %s"
	FROMAS = "FROM %s as %s"
	COMMON = `ENV UID {{.UID}}
ENV Appname {{.Appname}}
ENV Appdir {{.Appdir}}
ENV ERU 1
{{ if .Source }}ADD {{.Reponame}} {{.Appdir}}/{{.Appname}}{{ else }}RUN mkdir -p {{.Appdir}}/{{.Appname}}{{ end }}
WORKDIR {{.Appdir}}/{{.Appname}}
RUN useradd -u {{.UID}} -d /nonexistent -s /sbin/nologin -U {{.Appname}}
RUN chown -R {{.UID}} {{.Appdir}}/{{.Appname}}`
	RUN  = "RUN sh -c \"%s\""
	COPY = "COPY --from=%s %s %s"
	USER = "USER %s"
)

// richSpecs is used to format templates
type richSpecs struct {
	types.Specs
	UID      string
	Appdir   string
	Reponame string
	Source   bool
}

// Get a random node from pod `podname`
func getRandomNode(c *calcium, podname string) (*types.Node, error) {
	nodes, err := c.ListPodNodes(podname, false)
	if err != nil {
		return nil, err
	}
	if len(nodes) == 0 {
		err = fmt.Errorf("No nodes available in pod %s", podname)
		log.Debugf("Error during getRandomNode from %s: %v", podname, err)
		return nil, err
	}

	nodemap := make(map[string]types.CPUMap)
	for _, n := range nodes {
		nodemap[n.Name] = n.CPU
	}
	nodename, err := c.scheduler.RandomNode(nodemap)
	if err != nil {
		log.Debugf("Error during getRandomNode from %s: %v", podname, err)
		return nil, err
	}
	if nodename == "" {
		err = fmt.Errorf("Got empty node during getRandomNode from %s", podname)
		return nil, err
	}

	return c.GetNode(podname, nodename)
}

// BuildImage will build image for repository
// since we wanna set UID for the user inside container, we have to know the uid parameter
//
// build directory is like:
//
//    buildDir ├─ :appname ├─ code
//             ├─ Dockerfile
func (c *calcium) BuildImage(repository, version, uid, artifact string) (chan *types.BuildImageMessage, error) {
	ch := make(chan *types.BuildImageMessage)

	buildPodname := c.config.Docker.BuildPod
	if buildPodname == "" {
		return ch, fmt.Errorf("No build pod set in config")
	}

	node, err := getRandomNode(c, buildPodname)
	if err != nil {
		return ch, err
	}

	buildDir, err := ioutil.TempDir(os.TempDir(), "corebuild-")
	if err != nil {
		return ch, err
	}
	defer os.RemoveAll(buildDir)

	// parse repository name
	// code locates under /:repositoryname
	reponame, err := utils.GetGitRepoName(repository)
	if err != nil {
		return ch, err
	}

	// clone code into cloneDir
	// which is under buildDir and named as repository name
	cloneDir := filepath.Join(buildDir, reponame)
	if err := c.source.SourceCode(repository, cloneDir, version); err != nil {
		return ch, err
	}

	// ensure source code is safe
	// we don't want any history files to be retrieved
	if err := c.source.Security(cloneDir); err != nil {
		return ch, err
	}

	// use app.yaml file to create Specs instance
	// which we'll need to generate build args later
	bytes, err := ioutil.ReadFile(filepath.Join(cloneDir, "app.yaml"))
	if err != nil {
		return ch, err
	}
	specs := types.Specs{}
	if err := yaml.Unmarshal(bytes, &specs); err != nil {
		return ch, err
	}

	// if artifact download url is provided, remove all source code to
	// improve security
	if artifact != "" {
		os.RemoveAll(cloneDir)
		os.MkdirAll(cloneDir, os.ModeDir)
		if err := c.source.Artifact(artifact, cloneDir); err != nil {
			return ch, err
		}
	}

	// create dockerfile
	rs := richSpecs{specs, uid, strings.TrimRight(c.config.AppDir, "/"), reponame, true}
	if len(specs.Build) > 0 {
		if err := makeSimpleDockerFile(rs, buildDir); err != nil {
			return ch, err
		}
	} else {
		if err := makeComplexDockerFile(rs, buildDir); err != nil {
			return ch, err
		}
	}

	// tag of image, later this will be used to push image to hub
	tag := createImageTag(c.config, specs.Appname, utils.TruncateID(version))

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

	log.Infof("Building image %v with artifact %v at %v:%v", tag, artifact, buildPodname, node.Name)
	resp, err := node.Engine.ImageBuild(context.Background(), buildContext, buildOptions)
	if err != nil {
		return ch, err
	}

	go func() {
		defer resp.Body.Close()
		defer close(ch)
		decoder := json.NewDecoder(resp.Body)
		for {
			message := &types.BuildImageMessage{}
			err := decoder.Decode(message)
			if err != nil {
				if err == io.EOF {
					break
				}
				log.Errorf("Decode build image message failed %v", err)
				return
			}
			ch <- message
		}

		// About this "Khadgar", https://github.com/docker/docker/issues/10983#issuecomment-85892396
		// Just because Ben Schnetzer's cute Khadgar...
		rc, err := node.Engine.ImagePush(context.Background(), tag, enginetypes.ImagePushOptions{RegistryAuth: "Khadgar"})
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
				log.Errorf("Decode push image message failed %v", err)
				return
			}
			ch <- message
		}

		// 无论如何都删掉build机器的
		// 事实上他不会跟cached pod一样
		// 一样就砍死
		go node.Engine.ImageRemove(context.Background(), tag, enginetypes.ImageRemoveOptions{
			Force:         false,
			PruneChildren: true,
		})

		ch <- &types.BuildImageMessage{Status: "finished", Progress: tag}
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

func makeCommonPart(rs richSpecs) (string, error) {
	tmpl := template.Must(template.New("dockerfile").Parse(COMMON))
	out := bytes.Buffer{}
	if err := tmpl.Execute(&out, rs); err != nil {
		return "", err
	}
	return out.String(), nil
}

func makeMainPart(from, commands string, copys []string, rs richSpecs) (string, error) {
	var buildTmpl []string
	common, err := makeCommonPart(rs)
	if err != nil {
		return "", err
	}
	buildTmpl = append(buildTmpl, from, common)
	if len(copys) > 0 {
		buildTmpl = append(buildTmpl, copys...)
	}
	buildTmpl = append(buildTmpl, commands, "")
	return strings.Join(buildTmpl, "\n"), nil
}

func makeSimpleDockerFile(rs richSpecs, buildDir string) error {
	from := fmt.Sprintf(FROM, rs.Base)
	user := fmt.Sprintf(USER, rs.Appname)
	commands := fmt.Sprintf(RUN, strings.Join(rs.Build, " && "))
	// make sure add source code
	rs.Source = true
	mainPart, err := makeMainPart(from, commands, []string{}, rs)
	if err != nil {
		return err
	}
	dockerfile := fmt.Sprintf("%s\n%s", mainPart, user)
	return createDockerfile(dockerfile, buildDir)
}

func makeComplexDockerFile(rs richSpecs, buildDir string) error {
	var preArtifacts map[string]string
	var preStage string
	var buildTmpl []string

	for _, stage := range rs.ComplexBuild.Stages {
		build, ok := rs.ComplexBuild.Builds[stage]
		if !ok {
			log.Warnf("Complex build stage %s not defined", stage)
			continue
		}

		from := fmt.Sprintf(FROMAS, build.Base, stage)
		copys := []string{}
		for src, dst := range preArtifacts {
			copys = append(copys, fmt.Sprintf(COPY, preStage, src, dst))
		}
		commands := fmt.Sprintf(RUN, strings.Join(build.Commands, " && "))
		// decide add source or not
		rs.Source = build.Source
		mainPart, err := makeMainPart(from, commands, copys, rs)
		if err != nil {
			return err
		}
		buildTmpl = append(buildTmpl, mainPart)
		preStage = stage
		preArtifacts = build.Artifacts
	}
	buildTmpl = append(buildTmpl, fmt.Sprintf(USER, rs.Appname))
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
func createImageTag(config types.Config, appname, version string) string {
	prefix := strings.Trim(config.Docker.HubPrefix, "/")
	if prefix == "" {
		return fmt.Sprintf("%s/%s:%s", config.Docker.Hub, appname, version)
	}
	return fmt.Sprintf("%s/%s/%s:%s", config.Docker.Hub, prefix, appname, version)
}
