package calcium

import (
	"fmt"
	"io"
	"io/ioutil"
	"path/filepath"
	"strings"

	log "github.com/Sirupsen/logrus"
	enginetypes "github.com/docker/docker/api/types"
	enginecontainer "github.com/docker/docker/api/types/container"
	enginenetwork "github.com/docker/docker/api/types/network"
	engineapi "github.com/docker/docker/client"
	"gitlab.ricebook.net/platform/core/types"
	"gitlab.ricebook.net/platform/core/utils"
	"golang.org/x/net/context"
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
// app.yaml支持三种写法.
// app.yaml里可以支持mount_paths的写法, 例如
// mount_paths:
//     - "/var/www/html"
//     - "/data/eggsy"
// 这样的路径会被直接挂载到permdir下面去, 例如上面的路径就是
// /mnt/mfs/permdirs/eggsy/data/eggsy
// /mnt/mfs/permdirs/eggsy/var/www/html
// 而且这些路径是可以读写的.
//
// 或者使用volumes, 参数格式跟docker一样, 例如
// volumes:
//     - "/data/test:/test:ro"
//     - "/data/testx:/testx"
// 说明把宿主机的/data/test映射到容器里的/test, 只读, 同时
// 把宿主机的/data/tests映射到容器里的/testx, 读写.
//
// 或者使用binds, 例如
// binds:
//     "/host/path":
//         bind: "/container/path"
//         ro: true
// 说明把宿主机的/host/path映射到容器里的/container/path, 并且只读
func makeMountPaths(specs types.Specs, config types.Config) ([]string, map[string]struct{}) {
	binds := []string{}
	volumes := make(map[string]struct{})
	permDirHost := filepath.Join(config.PermDir, specs.Appname)

	// mount_paths
	for _, path := range specs.MountPaths {
		hostPath := filepath.Join(permDirHost, path)
		binds = append(binds, fmt.Sprintf("%s:%s:rw", hostPath, path))
		volumes[path] = struct{}{}
	}

	// volumes
	for _, path := range specs.Volumes {
		parts := strings.Split(path, ":")
		if len(parts) == 2 {
			binds = append(binds, fmt.Sprintf("%s:%s:ro", parts[0], parts[1]))
			volumes[parts[1]] = struct{}{}
		} else if len(parts) == 3 {
			binds = append(binds, fmt.Sprintf("%s:%s:%s", parts[0], parts[1], parts[2]))
			volumes[parts[1]] = struct{}{}
		}
	}

	// binds
	var mode string
	for hostPath, bind := range specs.Binds {
		if bind.ReadOnly {
			mode = "ro"
		} else {
			mode = "rw"
		}
		binds = append(binds, fmt.Sprintf("%s:%s:%s", hostPath, bind.InContainerPath, mode))
		volumes[bind.InContainerPath] = struct{}{}
	}

	// /proc/sys
	volumes["/writable-proc/sys"] = struct{}{}
	binds = append(binds, "/proc/sys:/writable-proc/sys:rw")
	binds = append(binds, "/sys/kernel/mm/transparent_hugepage:/writable-sys/kernel/mm/transparent_hugepage:rw")
	return binds, volumes
}

// 跑存在labels里的exec
// 为什么要存labels呢, 因为下线容器的时候根本不知道entrypoint是啥
func runExec(client *engineapi.Client, container enginetypes.ContainerJSON, label string) error {
	cmd, ok := container.Config.Labels[label]
	if !ok || cmd == "" {
		log.Debugf("No %s found in container %s", label, container.ID)
		return nil
	}

	cmds := utils.MakeCommandLineArgs(cmd)
	execConfig := enginetypes.ExecConfig{User: container.Config.User, Cmd: cmds}
	resp, err := client.ContainerExecCreate(context.Background(), container.ID, execConfig)
	if err != nil {
		log.Errorf("Error during runExec: %v", err)
		return err
	}

	return client.ContainerExecStart(context.Background(), resp.ID, enginetypes.ExecStartCheck{})
}
