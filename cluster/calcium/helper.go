package calcium

import (
	"archive/tar"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/docker/distribution/reference"
	enginetypes "github.com/docker/docker/api/types"
	enginecontainer "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	engineapi "github.com/docker/docker/client"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/registry"
	"github.com/projecteru2/core/cluster"
	"github.com/projecteru2/core/lock"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
)

// Lock is lock for calcium
func (c *Calcium) Lock(ctx context.Context, name string, timeout int) (lock.DistributedLock, error) {
	lock, err := c.store.CreateLock(name, timeout)
	if err != nil {
		return nil, err
	}
	if err = lock.Lock(ctx); err != nil {
		return nil, err
	}
	return lock, nil
}

// create container begin
func makeMemoryPriorSetting(memory int64, cpu float64) enginecontainer.Resources {
	resource := enginecontainer.Resources{}
	if cpu > 0 {
		resource.CPUPeriod = cluster.CPUPeriodBase
		resource.CPUQuota = int64(cpu * float64(cluster.CPUPeriodBase))
	}
	resource.Memory = memory
	resource.MemorySwap = memory
	return resource
}

func makeCPUPriorSetting(shareBase int64, quota types.CPUMap, memory int64) enginecontainer.Resources {
	// calculate CPUShares and CPUSet
	// scheduler won't return more than 1 share quota
	// so the smallest share is the share numerator
	shareQuota := shareBase
	cpuIDs := []string{}
	for cpuID, share := range quota {
		cpuIDs = append(cpuIDs, cpuID)
		if share < shareQuota {
			shareQuota = share
		}
	}
	cpuShares := int64(float64(shareQuota) / float64(shareBase) * float64(cluster.CPUShareBase))
	cpuSetCpus := strings.Join(cpuIDs, ",")
	log.Debugf("[makeCPUPriorSetting] CPU core %v CPU share %v Memory soft limit %v", cpuSetCpus, cpuShares, memory)
	resource := enginecontainer.Resources{
		CPUShares:         cpuShares,
		CpusetCpus:        cpuSetCpus,
		MemoryReservation: memory,
	}
	return resource
}

// image begin
// MakeAuthConfigFromRemote Calculate encoded AuthConfig from registry and eru-core config
// See https://github.com/docker/cli/blob/16cccc30f95c8163f0749eba5a2e80b807041342/cli/command/registry.go#L67
func makeEncodedAuthConfigFromRemote(authConfigs map[string]types.AuthConfig, remote string) (string, error) {
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
	if authConfig, exists := authConfigs[serverAddress]; exists {
		if encodedAuth, err := encodeAuthToBase64(authConfig); err == nil {
			return encodedAuth, nil
		}
		return "", err
	}
	return "dummy", nil
}

// EncodeAuthToBase64 serializes the auth configuration as JSON base64 payload
// See https://github.com/docker/cli/blob/master/cli/command/registry.go#L41
func encodeAuthToBase64(authConfig types.AuthConfig) (string, error) {
	buf, err := json.Marshal(authConfig)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(buf), nil
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
	//TODO 外部传入
	volumes["/writable-proc/sys"] = struct{}{}
	binds = append(binds, "/proc/sys:/writable-proc/sys:rw")
	volumes["/writable-sys/kernel/mm/transparent_hugepage"] = struct{}{}
	binds = append(binds, "/sys/kernel/mm/transparent_hugepage:/writable-sys/kernel/mm/transparent_hugepage:rw")
	return binds, volumes
}

// 跑存在labels里的exec
// 为什么要存labels呢, 因为下线容器的时候根本不知道entrypoint是啥
func execuateInside(ctx context.Context, client *engineapi.Client, ID, cmd, user string, env []string, privileged bool) ([]byte, error) {
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
	idResp, err := client.ContainerExecCreate(ctx, ID, execConfig)
	if err != nil {
		return []byte{}, err
	}
	resp, err := client.ContainerExecAttach(ctx, idResp.ID, enginetypes.ExecStartCheck{})
	if err != nil {
		return []byte{}, err
	}
	defer resp.Close()
	stream := utils.FuckDockerStream(ioutil.NopCloser(resp.Reader))
	b, err := ioutil.ReadAll(stream)
	if err != nil {
		return []byte{}, err
	}
	info, err := client.ContainerExecInspect(ctx, idResp.ID)
	if err != nil {
		return []byte{}, err
	}
	if info.ExitCode != 0 {
		return []byte{}, fmt.Errorf("%s", b)
	}
	return b, nil
}

// Pull an image
func pullImage(ctx context.Context, node *types.Node, image, auth string) error {
	log.Debugf("[pullImage] Pulling image %s", image)
	if image == "" {
		return fmt.Errorf("Goddamn empty image, WTF?")
	}

	pullOptions := enginetypes.ImagePullOptions{RegistryAuth: auth}
	outStream, err := node.Engine.ImagePull(ctx, image, pullOptions)
	if err != nil {
		log.Errorf("[pullImage] Error during pulling image %s: %v", image, err)
		return err
	}
	ensureReaderClosed(outStream)
	log.Debugf("[pullImage] Done pulling image %s", image)
	return nil
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

func createTarFileBuffer(filename string, data []byte) (io.Reader, error) {
	buf := &bytes.Buffer{}
	tw := tar.NewWriter(buf)
	defer tw.Close()
	hdr := &tar.Header{
		Name: filename,
		Mode: 0755,
		Size: int64(len(data)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return buf, err
	}
	if _, err := tw.Write(data); err != nil {
		return buf, err
	}
	return buf, nil
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

// 只要一个image的前面, tag不要
func normalizeImage(image string) string {
	if strings.Contains(image, ":") {
		t := strings.Split(image, ":")
		return t[0]
	}
	return image
}

// 清理一个node上的这个image
// 只清理同名字不同tag的
// 并且保留最新的 count 个
func cleanImageOnNode(ctx context.Context, node *types.Node, image string, count int) error {
	log.Debugf("[cleanImageOnNode] node: %s, image: %s", node.Name, strings.Split(image, ":")[0])
	imgListFilter := filters.NewArgs()
	image = normalizeImage(image)
	imgListFilter.Add("reference", image) // 相同repo的image
	images, err := node.Engine.ImageList(ctx, enginetypes.ImageListOptions{Filters: imgListFilter})
	if err != nil {
		return err
	}

	if len(images) < count {
		return nil
	}

	images = images[count:]
	log.Debugf("[cleanImageOnNode] Delete Images: %v", images)

	for _, image := range images {
		_, err := node.Engine.ImageRemove(ctx, image.ID, enginetypes.ImageRemoveOptions{
			Force:         false,
			PruneChildren: true,
		})
		if err != nil {
			log.Errorf("[cleanImageOnNode] Node %s ImageRemove error: %s, imageID: %s", node.Name, err, image.ID)
		}
	}
	return nil
}

func makeCopyMessage(id, status, name, path string, err error, data io.ReadCloser) *types.CopyMessage {
	return &types.CopyMessage{
		ID:     id,
		Status: status,
		Name:   name,
		Path:   path,
		Error:  err,
		Data:   data,
	}
}

func updateContainer(ctx context.Context, ID string, node *types.Node, config enginecontainer.UpdateConfig) error {
	_, err := node.Engine.ContainerUpdate(ctx, ID, config)
	return err
}

func filterNode(node *types.Node, labels map[string]string) bool {
	if node.Labels == nil && labels == nil {
		return true
	} else if node.Labels == nil && labels != nil {
		return false
	} else if node.Labels != nil && labels == nil {
		return true
	}

	for k, v := range labels {
		if d, ok := node.Labels[k]; !ok {
			return false
		} else if d != v {
			return false
		}
	}
	return true
}

func getNodesInfo(cpuAndMemData map[string]types.CPUAndMem) []types.NodeInfo {
	result := []types.NodeInfo{}
	for nodename, cpuAndMem := range cpuAndMemData {
		cpuRate := int64(len(cpuAndMem.CpuMap)) * cluster.CPUPeriodBase
		n := types.NodeInfo{
			CPUAndMem: cpuAndMem,
			Name:      nodename,
			CPURate:   cpuRate,
			Capacity:  0,
			Count:     0,
			Deploy:    0,
		}
		result = append(result, n)
	}
	return result
}

func parseStatusKey(key string) (string, string, string, string) {
	parts := strings.Split(key, "/")
	l := len(parts)
	return parts[l-4], parts[l-3], parts[l-2], parts[l-1]
}
