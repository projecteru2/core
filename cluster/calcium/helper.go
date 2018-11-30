package calcium

import (
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
	"github.com/docker/docker/registry"
	"github.com/projecteru2/core/cluster"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
)

func makeResourceSetting(cpu float64, memory int64, cpuMap types.CPUMap, softlimit bool) enginecontainer.Resources {
	resource := enginecontainer.Resources{}
	if cpu > 0 {
		resource.CPUPeriod = cluster.CPUPeriodBase
		resource.CPUQuota = int64(cpu * float64(cluster.CPUPeriodBase))
	}
	if cpuMap != nil && len(cpuMap) > 0 {
		cpuIDs := []string{}
		for cpuID := range cpuMap {
			cpuIDs = append(cpuIDs, cpuID)
		}
		resource.CpusetCpus = strings.Join(cpuIDs, ",")
	}
	if softlimit {
		resource.MemoryReservation = memory
	} else {
		resource.Memory = memory
		resource.MemorySwap = memory
		if memory/2 > minMemory {
			resource.MemoryReservation = memory / 2
		}
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

	return binds, volumes
}

func execuateInside(ctx context.Context, client engineapi.APIClient, ID, cmd, user string, env []string, privileged bool) ([]byte, error) {
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

func distributionInspect(ctx context.Context, node *types.Node, image, auth string, digests []string) bool {
	inspect, err := node.Engine.DistributionInspect(ctx, image, auth)
	if err != nil {
		log.Errorf("[distributionInspect] get manifest failed %v", err)
		return false
	}
	remoteDigest := fmt.Sprintf("%s@%s", normalizeImage(image), inspect.Descriptor.Digest.String())

	for _, digest := range digests {
		if digest == remoteDigest {
			log.Debugf("[distributionInspect] Local digest %s", digest)
			log.Debugf("[distributionInspect] Remote digest %s", remoteDigest)
			return true
		}
	}
	return false
}

// Pull an image
func pullImage(ctx context.Context, node *types.Node, image, auth string) error {
	log.Infof("[pullImage] Pulling image %s", image)
	if image == "" {
		return types.ErrNoImage
	}

	// check local
	exists := false
	inspect, _, err := node.Engine.ImageInspectWithRaw(ctx, image)
	if err != nil {
		log.Errorf("[pullImage] Check image failed %v", err)
	} else {
		log.Debug("[pullImage] Local Image exists")
		exists = true
	}

	if exists && distributionInspect(ctx, node, image, auth, inspect.RepoDigests) {
		log.Debug("[pullImage] Image cached, skip pulling")
		return nil
	}

	log.Info("[pullImage] Image not cached, pulling")
	pullOptions := enginetypes.ImagePullOptions{All: false, RegistryAuth: auth}
	outStream, err := node.Engine.ImagePull(ctx, image, pullOptions)
	if err != nil {
		log.Errorf("[pullImage] Error during pulling image %s: %v", image, err)
		return err
	}
	ensureReaderClosed(outStream)
	log.Infof("[pullImage] Done pulling image %s", image)
	return nil
}

func makeErrorBuildImageMessage(err error) *types.BuildImageMessage {
	return &types.BuildImageMessage{Error: err.Error()}
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
// 格式严格按照 Hub/HubPrefix/appname:tag 来
func createImageTag(config types.DockerConfig, appname, tag string) string {
	prefix := strings.Trim(config.Namespace, "/")
	if prefix == "" {
		return fmt.Sprintf("%s/%s:%s", config.Hub, appname, tag)
	}
	return fmt.Sprintf("%s/%s/%s:%s", config.Hub, prefix, appname, tag)
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
	for _, image := range images {
		log.Debugf("[cleanImageOnNode] Delete Images: %s", image.RepoTags)
		_, err := node.Engine.ImageRemove(ctx, image.ID, enginetypes.ImageRemoveOptions{
			Force:         false,
			PruneChildren: true,
		})
		if err != nil {
			log.Errorf("[cleanImageOnNode] Node %s ImageRemove error: %s, imageID: %s", node.Name, err, image.RepoTags)
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

func getNodesInfo(nodes map[string]*types.Node, cpu float64, memory int64) []types.NodeInfo {
	result := []types.NodeInfo{}
	for _, node := range nodes {
		nodeInfo := types.NodeInfo{
			Name:     node.Name,
			CPUMap:   node.CPU,
			MemCap:   node.MemCap,
			CPURate:  cpu / float64(len(node.InitCPU)),
			MemRate:  float64(memory) / float64(node.InitMemCap),
			CPUUsage: node.CPUUsage / float64(len(node.InitCPU)),
			MemUsage: 1.0 - float64(node.MemCap)/float64(node.InitMemCap),
			Capacity: 0,
			Count:    0,
			Deploy:   0,
		}
		result = append(result, nodeInfo)
	}
	return result
}
