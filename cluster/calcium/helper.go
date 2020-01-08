package calcium

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"strconv"
	"strings"

	"bufio"

	"github.com/projecteru2/core/engine"
	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
)

var winchCommand = []byte{0x80}  // 128, non-ASCII
var escapeCommand = []byte{0x1d} // 29, ^]

const (
	AUTO = "AUTO"
)

type window struct {
	Height uint `json:"Row"`
	Width  uint `json:"Col"`
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

	outStream, _, err := client.ExecAttach(ctx, execID, false)
	if err != nil {
		return []byte{}, err
	}

	b := []byte{}
	for data := range processVirtualizationOutStream(ctx, outStream) {
		b = append(b, data...)
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

func getNodesInfo(nodes map[string]*types.Node, cpu float64, memory, storage int64) []types.NodeInfo {
	result := []types.NodeInfo{}
	for _, node := range nodes {
		nodeInfo := types.NodeInfo{
			Name:         node.Name,
			CPUMap:       node.CPU,
			VolumeMap:    node.Volume,
			MemCap:       node.MemCap,
			StorageCap:   node.AvailableStorage(),
			CPURate:      cpu / float64(len(node.InitCPU)),
			MemRate:      float64(memory) / float64(node.InitMemCap),
			StorageRate:  float64(storage) / float64(node.InitStorageCap),
			CPUUsed:      node.CPUUsed / float64(len(node.InitCPU)),
			VolumeUsed:   float64(node.VolumeUsed) / float64(len(node.InitVolume)),
			MemUsage:     1.0 - float64(node.MemCap)/float64(node.InitMemCap),
			StorageUsage: node.StorageUsage(),
			Capacity:     0,
			Count:        0,
			Deploy:       0,
		}
		result = append(result, nodeInfo)
	}
	return result
}

func processVirtualizationInStream(
	ctx context.Context,
	inStream io.WriteCloser,
	inCh <-chan []byte,
	resizeFunc func(height, width uint) error,
) <-chan struct{} {
	specialPrefixCallback := map[string]func([]byte){
		string(winchCommand): func(body []byte) {
			w := &window{}
			if err := json.Unmarshal(body, w); err != nil {
				log.Errorf("[processVirtualizationInStream] invalid winch command: %q", body)
				return
			}
			if err := resizeFunc(w.Height, w.Width); err != nil {
				log.Errorf("[processVirtualizationInStream] resize window error: %v", err)
				return
			}
			return
		},

		string(escapeCommand): func(body []byte) {
			inStream.Close()
		},
	}
	return rawProcessVirtualizationInStream(ctx, inStream, inCh, specialPrefixCallback)
}

func rawProcessVirtualizationInStream(
	ctx context.Context,
	inStream io.WriteCloser,
	inCh <-chan []byte,
	specialPrefixCallback map[string]func([]byte),
) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer inStream.Close()

		for cmd := range inCh {
			cmdKey := string(cmd[:1])
			if f, ok := specialPrefixCallback[cmdKey]; ok {
				f(cmd[1:])
				continue
			}
			if _, err := inStream.Write(cmd); err != nil {
				log.Errorf("[rawProcessVirtualizationInStream] failed to write virtual input stream: %v", err)
				return
			}
		}
	}()

	return done
}

func processVirtualizationOutStream(
	ctx context.Context,
	outStream io.ReadCloser,
) <-chan []byte {
	outCh := make(chan []byte)
	go func() {
		defer outStream.Close()
		defer close(outCh)
		scanner := bufio.NewScanner(outStream)
		scanner.Split(bufio.ScanBytes)
		for scanner.Scan() {
			b := scanner.Bytes()
			outCh <- b
		}
		if err := scanner.Err(); err != nil {
			log.Errorf("[processVirtualizationOutStream] failed to read output from output stream: %v", err)
		}
		return
	}()
	return outCh
}

func mergeAutoVolumeRequests(volumes1 []string, volumes2 []string) (volumes []string, err error) {
	sizeMap := map[string]int64{} // {"AUTO:/data:rw": 100}
	for _, vol := range append(volumes1, volumes2...) {
		parts := strings.Split(vol, ":")
		if len(parts) != 4 || parts[0] != AUTO {
			continue
		}
		size, err := strconv.ParseInt(parts[3], 10, 64)
		if err != nil {
			return nil, err
		}
		prefix := strings.Join(parts[0:3], ":")
		if _, ok := sizeMap[prefix]; ok {
			sizeMap[prefix] += size
		} else {
			sizeMap[prefix] = size
		}
	}

	for prefix, size := range sizeMap {
		if size < 0 {
			continue
		}
		volumes = append(volumes, fmt.Sprintf("%s:%d", prefix, size))
	}
	return volumes, nil
}
