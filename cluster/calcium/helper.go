package calcium

import (
	"fmt"
	"io"
	"io/ioutil"
	"path/filepath"
	"strings"

	enginetypes "github.com/docker/engine-api/types"
	enginecontainer "github.com/docker/engine-api/types/container"
	enginenetwork "github.com/docker/engine-api/types/network"
	"gitlab.ricebook.net/platform/core/utils"
)

// As the name says,
// blocks until the stream is empty, until we meet EOF
func ensureReaderClosed(stream io.ReadCloser) {
	if stream == nil {
		return
	}
	io.Copy(ioutil.Discard, stream)
	stream.Close()
}

// Copies config from container
// And make a new name for container
func makeContainerConfig(info enginetypes.ContainerJSON, image string) (
	*enginecontainer.Config,
	*enginecontainer.HostConfig,
	*enginenetwork.NetworkingConfig,
	string,
	error) {

	// we use `_` to join container name
	// since we don't support `_` in entrypoint, and no `_` is in suffix,
	// the last part will be suffix and second last part will be entrypoint,
	// the rest will be the appname
	parts := strings.Split(trimLeftSlash(info.Name), "_")
	length := len(parts)
	if length < 3 {
		return nil, nil, nil, "", fmt.Errorf("Bad container name format: %q", info.Name)
	}

	entrypoint := parts[length-2]
	appname := strings.Join(parts[:length-2], "_")

	suffix := utils.RandomString(6)
	containerName := strings.Join([]string{appname, entrypoint, suffix}, "_")

	config := info.Config
	config.Image = image

	hostConfig := info.HostConfig
	networkConfig := &enginenetwork.NetworkingConfig{
		EndpointsConfig: info.NetworkSettings.Networks,
	}
	return config, hostConfig, networkConfig, containerName, nil
}

// see https://github.com/docker/docker/issues/6705
// docker's stupid problem
func trimLeftSlash(name string) string {
	return strings.TrimPrefix(name, "/")
}

// make mount paths
// app.yaml里可以支持mount_paths的写法, 例如
// mount_paths:
//     - "/var/www/html"
//     - "/data/eggsy"
// 这样的路径会被直接挂载到permdir下面去, 例如上面的路径就是
// /mnt/mfs/permdirs/eggsy/data/eggsy
// /mnt/mfs/permdirs/eggsy/var/www/html
// 而且这些路径是可以读写的.
func makeMountPaths(paths []string, permDirHost string) ([]string, map[string]struct{}) {
	binds := []string{}
	volumes := make(map[string]struct{})
	for _, path := range paths {
		hostPath := filepath.Join(permDirHost, path)
		binds = append(binds, fmt.Sprintf("%s:%s:rw", hostPath, path))
		volumes[path] = struct{}{}
	}
	return binds, volumes
}
