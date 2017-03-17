package calcium

import (
	"encoding/json"
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
	"gitlab.ricebook.net/platform/core/types"
	"gitlab.ricebook.net/platform/core/utils"
	"golang.org/x/net/context"
	"gopkg.in/yaml.v2"
)

const launcherScript = `#! /bin/sh
echo 32768 > /writable-proc/sys/net/core/somaxconn
echo 1 > /writable-proc/sys/vm/overcommit_memory
chmod 777 /dev/stdout
chmod 777 /dev/stderr
if [ -d {{.Appdir}}/{{.Appname}}/permdir ]; then chown {{.UID}} {{.Appdir}}/{{.Appname}}/permdir; fi

neednetwork=$1
if [ $neednetwork = "network" ]; then
    # wait for macvlan
    while ( ! ip addr show | grep 'UP' | grep 'vnbe'); do
        echo -n o
        sleep .5
    done
fi

sleep 1

shift

{{.Command}}
`

const dockerFile = `FROM {{.Base}}
ENV ERU 1
ADD %s {{.Appdir}}/{{.Appname}}
ADD launcher /usr/local/bin/launcher
ADD launcheroot /usr/local/bin/launcheroot
WORKDIR {{.Appdir}}/{{.Appname}}
RUN useradd -u %s -d /nonexistent -s /sbin/nologin -U {{.Appname}}
RUN chown -R %s {{.Appdir}}/{{.Appname}}
{{with .Build}}
{{range $index, $value := .}}
RUN {{$value}}
{{end}}
{{end}}
`

// richSpecs is used to format templates
type richSpecs struct {
	types.Specs
	Command string
	Appdir  string
	UID     string
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
//             ├─ launcher
//             ├─ launcheroot
func (c *calcium) BuildImage(repository, version, uid, artifact string) (chan *types.BuildImageMessage, error) {
	ch := make(chan *types.BuildImageMessage)

	buildPodname := c.config.Docker.BuildPod
	if buildPodname == "" {
		// use pod `dev` to build image as default
		buildPodname = "dev"
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

	// ensure .git directory is removed
	// we don't want any history files to be retrieved
	if err := os.RemoveAll(filepath.Join(cloneDir, ".git")); err != nil {
		log.Errorf("Error when removing .git dir")
		return ch, err
	}

	// use app.yaml file to create Specs instance
	// which we'll need to create Dockerfile later
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
			log.Errorf("Error when downloading artifact: %s", err.Error())
		}
	}

	// create launcher scripts and dockerfile
	rs := richSpecs{specs, "", strings.TrimRight(c.config.AppDir, "/"), uid}
	if err := createLauncher(buildDir, rs); err != nil {
		return ch, err
	}
	if err := createDockerfile(buildDir, reponame, rs); err != nil {
		return ch, err
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
		decoder := json.NewDecoder(resp.Body)
		for {
			message := &types.BuildImageMessage{}
			err := decoder.Decode(message)
			if err != nil {
				if err == io.EOF {
					break
				} else {
					close(ch)
					return
				}
			}
			ch <- message
		}

		// About this "Khadgar", https://github.com/docker/docker/issues/10983#issuecomment-85892396
		// Just because Ben Schnetzer's cute Khadgar...
		rc, err := node.Engine.ImagePush(context.Background(), tag, enginetypes.ImagePushOptions{RegistryAuth: "Khadgar"})
		if err != nil {
			ch <- makeErrorBuildImageMessage(err)
			close(ch)
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
				} else {
					close(ch)
					return
				}
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
		close(ch)
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

// launcher scripts
func createLauncher(buildDir string, rs richSpecs) error {
	launcherScriptTemplate, _ := template.New("launcher script").Parse(launcherScript)

	entryCommand := fmt.Sprintf("exec sudo -E -u %s $@", rs.Appname)
	entryRootCommand := "exec $@"

	f, err := os.Create(filepath.Join(buildDir, "launcher"))
	if err != nil {
		return err
	}
	defer f.Close()
	rs.Command = entryCommand
	launcherScriptTemplate.Execute(f, rs)
	if err := f.Sync(); err != nil {
		return err
	}
	if err := f.Chmod(0755); err != nil {
		return err
	}

	fr, err := os.Create(filepath.Join(buildDir, "launcheroot"))
	if err != nil {
		return err
	}
	defer fr.Close()
	rs.Command = entryRootCommand
	launcherScriptTemplate.Execute(fr, rs)
	if err := fr.Sync(); err != nil {
		return err
	}
	if err := fr.Chmod(0755); err != nil {
		return err
	}

	return nil
}

// Dockerfile
func createDockerfile(buildDir, reponame string, rs richSpecs) error {
	f, err := os.Create(filepath.Join(buildDir, "Dockerfile"))
	if err != nil {
		return err
	}
	defer f.Close()

	dockerFileFormatted := fmt.Sprintf(dockerFile, reponame, rs.UID, rs.UID)
	t := template.New("docker file template")
	parsedTemplate, err := t.Parse(dockerFileFormatted)
	if err != nil {
		return err
	}
	err = parsedTemplate.Execute(f, rs)
	if err != nil {
		return err
	}

	if err := f.Sync(); err != nil {
		return err
	}
	return nil
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
