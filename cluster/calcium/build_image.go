package calcium

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"text/template"

	"github.com/docker/docker/pkg/archive"
	enginetypes "github.com/docker/engine-api/types"
	"gitlab.ricebook.net/platform/core/git"
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
ADD %s /{{.Appname}}
ADD launcher /usr/local/bin/launcher
ADD launcheroot /usr/local/bin/launcheroot
WORKDIR /{{.Appname}}
RUN useradd -u %s -d /nonexistent -s /sbin/nologin -U {{.Appname}}
{{with .Build}}
{{range $index, $value := .}}
RUN {{$value}}
{{end}}
{{end}}
`

// Entry is used to format templates
type entry struct {
	Command string
}

// Get a random node from pod `podname`
func getRandomNode(c *Calcium, podname string) (*types.Node, error) {
	nodes, err := c.ListPodNodes(podname)
	if err != nil {
		return nil, err
	}

	nodemap := make(map[string]types.CPUMap)
	for _, n := range nodes {
		nodemap[n.Name] = n.CPU
	}
	nodename, err := c.scheduler.RandomNode(nodemap)
	if err != nil {
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
func (c *Calcium) BuildImage(repository, version, uid string) (chan *types.BuildImageMessage, error) {
	ch := make(chan *types.BuildImageMessage)

	// use pod `dev` to build image
	node, err := getRandomNode(c, "dev")
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
	if err := git.CloneRepository(repository, cloneDir, version, c.config.Git.PublicKey, c.config.Git.PrivateKey); err != nil {
		return ch, err
	}

	// ensure .git directory is removed
	// we don't want any history files to be retrieved
	if err := os.RemoveAll(filepath.Join(cloneDir, ".git")); err != nil {
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

	// create launcher scripts and dockerfile
	if err := createLauncher(buildDir, specs); err != nil {
		return ch, err
	}
	if err := createDockerfile(buildDir, uid, reponame, specs); err != nil {
		return ch, err
	}

	// tag of image, later this will be used to push image to hub
	tag := fmt.Sprintf("%s/%s:%s", c.config.Docker.Hub, specs.Appname, utils.TruncateID(version))

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
				close(ch)
				break
			}
			ch <- message
		}
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
func createLauncher(buildDir string, specs types.Specs) error {
	launcherScriptTemplate, _ := template.New("launcher script").Parse(launcherScript)

	entryCommand := fmt.Sprintf("exec sudo -E -u %s $@", specs.Appname)
	entryRootCommand := "exec $@"

	f, err := os.Create(filepath.Join(buildDir, "launcher"))
	if err != nil {
		return err
	}
	defer f.Close()
	launcherScriptTemplate.Execute(f, entry{Command: entryCommand})
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
	launcherScriptTemplate.Execute(fr, entry{Command: entryRootCommand})
	if err := fr.Sync(); err != nil {
		return err
	}
	if err := fr.Chmod(0755); err != nil {
		return err
	}

	return nil
}

// Dockerfile
func createDockerfile(buildDir, uid, reponame string, specs types.Specs) error {
	f, err := os.Create(filepath.Join(buildDir, "Dockerfile"))
	if err != nil {
		return err
	}
	defer f.Close()

	dockerFileFormatted := fmt.Sprintf(dockerFile, reponame, uid)
	t := template.New("docker file template")
	parsedTemplate, err := t.Parse(dockerFileFormatted)
	if err != nil {
		return err
	}
	err = parsedTemplate.Execute(f, specs)
	if err != nil {
		return err
	}

	if err := f.Sync(); err != nil {
		return err
	}
	return nil
}
