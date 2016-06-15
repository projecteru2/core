package cluster

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/docker/docker/pkg/archive"
	enginetypes "github.com/docker/engine-api/types"
	"gitlab.ricebook.net/platform/core/git"
	"gitlab.ricebook.net/platform/core/types"
	"gitlab.ricebook.net/platform/core/utils"
	"golang.org/x/net/context"
	"gopkg.in/yaml.v2"
)

// build image for repository
// since we wanna set UID for the user inside container, we have to know the uid parameter
//
// build directory is like:
//
//    buildDir ├─ :appname ├─ code
//             ├─ Dockerfile
//             ├─ launcher
//             ├─ launcheroot
func (c *Calcium) BuildImage(repository, version, uid string) (chan *types.BuildImageMessage, error) {
	var node *types.Node
	ch := make(chan *types.BuildImageMessage)

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
	if err := git.CloneRepository(repository, cloneDir, version, c.config.Git.GitPublicKey, c.config.Git.GitPrivateKey); err != nil {
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
	specs := types.AppSpecs{}
	if err := yaml.Unmarshal(bytes, &specs); err != nil {
		return ch, err
	}

	// create launcher scripts and dockerfile
	if err := createLauncher(buildDir, specs); err != nil {
		return err
	}
	if err := createDockerfile(buildDir, uid, reponame, specs); err != nil {
		return err
	}

	// create tar stream for Build API
	buildContext, err := createTarStream(buildDir)
	if err != nil {
		return ch, err
	}

	// create options for Build API
	// here we use first 7 chars as tag
	buildOptions := enginetypes.ImageBuildOptions{
		Tags:           []string{utils.TruncateID(version)},
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
		decoder := json.NewDecoder(resp.Body)
		for {
			message := &types.BuildImageMessage{}
			err := decoder.Decode(message)
			if err == io.EOF {
				close(ch)
				break
			}
			// FIXME 似乎用途上一样?
			if err != nil {
				close(ch)
				return
			}
			ch <- message

		}
	}()

	return ch, nil
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
// TODO use golang template
func createLauncher(buildDir string, specs types.AppSpecs) error {
	entry := fmt.Sprintf("exec sudo -E -u %s $@", specs.Appname)
	entryRoot := "exec $@"

	tmpl := `#! /bin/sh
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

`
	f, err := os.Create(path.Join(buildDir, "launcher"))
	if err != nil {
		return err
	}
	defer f.Close()
	f.WriteString(tmpl)
	f.WriteString(entry)
	if err := f.Sync(); err != nil {
		return err
	}

	fr, err := os.Create(path.Join(buildDir, "launcheroot"))
	if err != nil {
		return err
	}
	defer fr.Close()
	fr.WriteString(tmpl)
	fr.WriteString(entryRoot)
	if err := fr.Sync(); err != nil {
		return err
	}

	return nil
}

// Dockerfile
func createDockerfile(buildDir, uid, reponame string, specs types.AppSpecs) error {
	f, err := os.Create(path.Join(buildDir, "Dockerfile"))
	if err != nil {
		return err
	}
	defer f.Close()

	f.WriteString(fmt.Sprintf("FROM %s\n", specs.Base))
	f.WriteString("ENV ERU 1\n")
	f.WriteString(fmt.Sprintf("ADD %s /%s\n", reponame, specs.Appname))
	f.WriteString("ADD launcher /user/local/bin/launcher\n")
	f.WriteString("ADD launcheroot /user/local/bin/launcheroot\n")
	f.WriteString(fmt.Sprintf("WORKDIR /%s\n", specs.Appname))
	f.WriteString(fmt.Sprintf("RUN useradd -u %d -d /nonexistent -s /sbin/nologin -U %s\n", uid, specs.Appname))
	for _, cmd := range specs.Build {
		f.WriteString(fmt.Sprintf("RUN %s\n", cmd))
	}

	if err := f.Sync(); err != nil {
		return err
	}
	return nil
}
