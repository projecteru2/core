package calcium

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	log "github.com/Sirupsen/logrus"
	enginetypes "github.com/docker/docker/api/types"
	enginecontainer "github.com/docker/docker/api/types/container"
	enginenetwork "github.com/docker/docker/api/types/network"
	engineapi "github.com/docker/docker/client"
	"github.com/projecteru2/core/lock"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
	"golang.org/x/net/context"
)

func (c *calcium) makeMemoryPriorSetting(memory int64, cpu float64) enginecontainer.Resources {
	resource := enginecontainer.Resources{
		Memory:     memory,
		MemorySwap: memory,
		CPUPeriod:  utils.CpuPeriodBase,
		CPUQuota:   int64(cpu * float64(utils.CpuPeriodBase)),
	}
	return resource
}

func (c *calcium) makeCPUPriorSetting(quota types.CPUMap) enginecontainer.Resources {
	// calculate CPUShares and CPUSet
	// scheduler won't return more than 1 share quota
	// so the smallest share is the share numerator
	shareQuota := c.config.Scheduler.ShareBase
	cpuids := []string{}
	for cpuid, share := range quota {
		cpuids = append(cpuids, cpuid)
		if share < shareQuota {
			shareQuota = share
		}
	}
	cpuShares := int64(float64(shareQuota) / float64(c.config.Scheduler.ShareBase) * float64(utils.CpuShareBase))
	cpuSetCpus := strings.Join(cpuids, ",")
	resource := enginecontainer.Resources{
		CPUShares:  cpuShares,
		CpusetCpus: cpuSetCpus,
	}
	return resource
}

func (c *calcium) calculateCPUUsage(container *types.Container) float64 {
	var full, fragment int64
	for _, usage := range container.CPU {
		if usage == c.config.Scheduler.ShareBase {
			full++
			continue
		}
		fragment += usage
	}
	return float64(full) + float64(fragment)/float64(c.config.Scheduler.ShareBase)
}

func (c *calcium) Lock(name string, timeout int) (lock.DistributedLock, error) {
	lock, err := c.store.CreateLock(name, timeout)
	if err != nil {
		return nil, err
	}
	if err = lock.Lock(); err != nil {
		return nil, err
	}
	return lock, nil
}

func makeCPUAndMem(nodes []*types.Node) map[string]types.CPUAndMem {
	r := make(map[string]types.CPUAndMem)
	for _, node := range nodes {
		r[node.Name] = types.CPUAndMem{
			CpuMap: node.CPU,
			MemCap: node.MemCap,
		}
	}
	return r
}

// filter nodes
// public is the flag
func filterNodes(nodes []*types.Node, public bool) []*types.Node {
	rs := []*types.Node{}
	for _, node := range nodes {
		if node.Public == public {
			rs = append(rs, node)
		}
	}
	return rs
}

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
// 使用volumes, 参数格式跟docker一样, 支持 $PERMDIR $APPDIR 的展开
// volumes:
//     - "$PERMDIR/foo-data:$APPDIR/foodata:rw"
func makeMountPaths(specs types.Specs, config types.Config) ([]string, map[string]struct{}) {
	binds := []string{}
	volumes := make(map[string]struct{})

	var expandENV = func(env string) string {
		envMap := make(map[string]string)
		if config.PermDir != "" {
			envMap["PERMDIR"] = filepath.Join(config.PermDir, specs.Appname)
		}
		envMap["APPDIR"] = filepath.Join(config.AppDir, specs.Appname)
		return envMap[env]
	}

	// volumes
	for _, path := range specs.Volumes {
		expanded := os.Expand(path, expandENV)
		parts := strings.Split(expanded, ":")
		if len(parts) == 2 {
			binds = append(binds, fmt.Sprintf("%s:%s:rw", parts[0], parts[1]))
			volumes[parts[1]] = struct{}{}
		} else if len(parts) == 3 {
			binds = append(binds, fmt.Sprintf("%s:%s:%s", parts[0], parts[1], parts[2]))
			volumes[parts[1]] = struct{}{}
		}
	}

	// /proc/sys
	volumes["/writable-proc/sys"] = struct{}{}
	binds = append(binds, "/proc/sys:/writable-proc/sys:rw")
	volumes["/writable-sys/kernel/mm/transparent_hugepage"] = struct{}{}
	binds = append(binds, "/sys/kernel/mm/transparent_hugepage:/writable-sys/kernel/mm/transparent_hugepage:rw")
	return binds, volumes
}

// 跑存在labels里的exec
// 为什么要存labels呢, 因为下线容器的时候根本不知道entrypoint是啥
func runExec(client *engineapi.Client, container enginetypes.ContainerJSON, label string) error {
	cmd, ok := container.Config.Labels[label]
	if !ok || cmd == "" {
		log.Debugf("[runExec] No %s found in container %s", label, container.ID)
		return nil
	}

	cmds := utils.MakeCommandLineArgs(cmd)
	execConfig := enginetypes.ExecConfig{User: container.Config.User, Cmd: cmds}
	resp, err := client.ContainerExecCreate(context.Background(), container.ID, execConfig)
	if err != nil {
		log.Errorf("[runExec] Error during runExec: %v", err)
		return err
	}
	return client.ContainerExecStart(context.Background(), resp.ID, enginetypes.ExecStartCheck{})
}
