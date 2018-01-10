package calcium

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"

	"github.com/docker/distribution/reference"
	enginetypes "github.com/docker/docker/api/types"
	enginecontainer "github.com/docker/docker/api/types/container"
	engineapi "github.com/docker/docker/client"
	"github.com/docker/docker/registry"
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

// As the name says,
// blocks until the stream is empty, until we meet EOF
func ensureReaderClosed(stream io.ReadCloser) {
	if stream == nil {
		return
	}
	io.Copy(ioutil.Discard, stream)
	stream.Close()
}

// make mount paths
// 使用volumes, 参数格式跟docker一样
// volumes:
//     - "/foo-data:$SOMEENV/foodata:rw"
func makeMountPaths(opts *types.DeployOptions) ([]string, map[string]struct{}) {
	binds := []string{}
	volumes := make(map[string]struct{})

	var expandENV = func(env string) string {
		envMap := map[string]string{}
		for _, env := range opts.Env {
			parts := strings.Split(env, "=")
			envMap[parts[0]] = parts[1]
		}
		return envMap[env]
	}

	for _, path := range opts.Volumes {
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
func execuateInside(client *engineapi.Client, ID, cmd, user string, env []string, privileged bool) ([]byte, error) {
	cmds := utils.MakeCommandLineArgs(cmd)
	execConfig := enginetypes.ExecConfig{
		User:         user,
		Cmd:          cmds,
		Privileged:   privileged,
		Env:          env,
		AttachStderr: true,
		AttachStdout: true,
	}
	//TODO should timeout
	//Fuck docker, ctx will not use inside funcs!!
	idResp, err := client.ContainerExecCreate(context.Background(), ID, execConfig)
	if err != nil {
		return []byte{}, err
	}
	resp, err := client.ContainerExecAttach(context.Background(), idResp.ID, execConfig)
	if err != nil {
		return []byte{}, err
	}
	defer resp.Close()
	stream := utils.FuckDockerStream(ioutil.NopCloser(resp.Reader))
	b, err := ioutil.ReadAll(stream)
	if err != nil {
		return []byte{}, err
	}
	info, err := client.ContainerExecInspect(context.Background(), idResp.ID)
	if err != nil {
		return []byte{}, err
	}
	if info.ExitCode != 0 {
		return []byte{}, fmt.Errorf("%s", b)
	}
	return b, nil
}

// MakeAuthConfigFromRemote Calculate encoded AuthConfig from registry and eru-core config
// See https://github.com/docker/cli/blob/16cccc30f95c8163f0749eba5a2e80b807041342/cli/command/registry.go#L67
func (c *calcium) MakeEncodedAuthConfigFromRemote(remote string) (string, error) {
	ref, err := reference.ParseNormalizedNamed(remote)
	if err != nil {
		return "", err
	}

	// Resolve the Repository name from fqn to RepositoryInfo
	repoInfo, err := registry.ParseRepositoryInfo(ref)
	if err != nil {
		return "", err
	}

	serverAddress := repoInfo.Index.Name
	if authConfig, exists := c.config.Docker.AuthConfigs[serverAddress]; exists {
		if encodedAuth, err := EncodeAuthToBase64(authConfig); err == nil {
			return encodedAuth, nil
		}
		return "", err
	}
	return "dummy", nil
}

// EncodeAuthToBase64 serializes the auth configuration as JSON base64 payload
// See https://github.com/docker/cli/blob/master/cli/command/registry.go#L41
func EncodeAuthToBase64(authConfig types.AuthConfig) (string, error) {
	buf, err := json.Marshal(authConfig)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(buf), nil
}
