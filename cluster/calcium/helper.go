package calcium

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/projecteru2/core/engine"
	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
	log "github.com/sirupsen/logrus"
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

// make mount paths
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

func execuateInside(ctx context.Context, client engine.API, ID, cmd, user string, env []string, privileged bool) ([]byte, error) {
	cmds := utils.MakeCommandLineArgs(cmd)
	execConfig := &enginetypes.ExecConfig{
		User:         user,
		Cmd:          cmds,
		Privileged:   privileged,
		Env:          env,
		AttachStderr: true,
		AttachStdout: true,
	}
	execID, err := client.ExecCreate(ctx, ID, execConfig)
	if err != nil {
		return []byte{}, err
	}

	resp, err := client.ExecAttach(ctx, execID, false, false)
	if err != nil {
		return []byte{}, err
	}
	defer resp.Close()

	b, err := ioutil.ReadAll(resp)
	if err != nil {
		return []byte{}, err
	}

	exitCode, err := client.ExecExitCode(ctx, execID)
	if err != nil {
		return b, err
	}
	if exitCode != 0 {
		return b, fmt.Errorf("%s", b)
	}
	return b, nil
}

func distributionInspect(ctx context.Context, node *types.Node, image string, digests []string) bool {
	remoteDigest, err := node.Engine.ImageRemoteDigest(ctx, image)
	if err != nil {
		log.Errorf("[distributionInspect] get manifest failed %v", err)
		return false
	}

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
func pullImage(ctx context.Context, node *types.Node, image string) error {
	log.Infof("[pullImage] Pulling image %s", image)
	if image == "" {
		return types.ErrNoImage
	}

	// check local
	exists := false
	digests, err := node.Engine.ImageLocalDigests(ctx, image)
	if err != nil {
		log.Errorf("[pullImage] Check image failed %v", err)
	} else {
		log.Debug("[pullImage] Local Image exists")
		exists = true
	}

	if exists && distributionInspect(ctx, node, image, digests) {
		log.Debug("[pullImage] Image cached, skip pulling")
		return nil
	}

	log.Info("[pullImage] Image not cached, pulling")
	outStream, err := node.Engine.ImagePull(ctx, image, false)
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

// 清理一个node上的这个image
// 只清理同名字不同tag的
// 并且保留最新的 count 个
func cleanImageOnNode(ctx context.Context, node *types.Node, image string, count int) error {
	log.Debugf("[cleanImageOnNode] node: %s, image: %s", node.Name, image)
	images, err := node.Engine.ImageList(ctx, image)
	if err != nil {
		return err
	}

	if len(images) < count {
		return nil
	}

	images = images[count:]
	for _, image := range images {
		log.Debugf("[cleanImageOnNode] Delete Images: %s", image.Tags)
		if _, err := node.Engine.ImageRemove(ctx, image.ID, false, true); err != nil {
			log.Errorf("[cleanImageOnNode] Node %s ImageRemove error: %s, imageID: %s", node.Name, err, image.Tags)
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
			CPUUsed:  node.CPUUsed / float64(len(node.InitCPU)),
			MemUsage: 1.0 - float64(node.MemCap)/float64(node.InitMemCap),
			Capacity: 0,
			Count:    0,
			Deploy:   0,
		}
		result = append(result, nodeInfo)
	}
	return result
}
