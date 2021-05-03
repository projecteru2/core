package calcium

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"sync"

	"github.com/pkg/errors"
	"github.com/projecteru2/core/engine"
	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
	"golang.org/x/net/context"
)

var winchCommand = []byte{0x80}  // 128, non-ASCII
var escapeCommand = []byte{0x1d} // 29, ^]

type window struct {
	Height uint `json:"Row"`
	Width  uint `json:"Col"`
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
	b := []byte{}
	execID, stdout, stderr, _, err := client.Execute(ctx, ID, execConfig)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	for m := range processStdStream(ctx, stdout, stderr, bufio.ScanLines, byte('\n')) {
		b = append(b, m.Data...)
	}

	exitCode, err := client.ExecExitCode(ctx, execID)
	if err != nil {
		return b, errors.WithStack(err)
	}
	if exitCode != 0 {
		return b, errors.WithStack(fmt.Errorf("%s", b))
	}
	return b, nil
}

func distributionInspect(ctx context.Context, node *types.Node, image string, digests []string) bool {
	remoteDigest, err := node.Engine.ImageRemoteDigest(ctx, image)
	if err != nil {
		log.Errorf(ctx, "[distributionInspect] get manifest failed %v", err)
		return false
	}

	for _, digest := range digests {
		if digest == remoteDigest {
			log.Debugf(ctx, "[distributionInspect] Local digest %s", digest)
			log.Debugf(ctx, "[distributionInspect] Remote digest %s", remoteDigest)
			return true
		}
	}
	return false
}

// Pull an image
func pullImage(ctx context.Context, node *types.Node, image string) error {
	log.Infof(ctx, "[pullImage] Pulling image %s", image)
	if image == "" {
		return errors.WithStack(types.ErrNoImage)
	}

	// check local
	exists := false
	digests, err := node.Engine.ImageLocalDigests(ctx, image)
	if err != nil {
		log.Errorf(ctx, "[pullImage] Check image failed %v", err)
	} else {
		log.Debug(ctx, "[pullImage] Local Image exists")
		exists = true
	}

	if exists && distributionInspect(ctx, node, image, digests) {
		log.Debug(ctx, "[pullImage] Image cached, skip pulling")
		return nil
	}

	log.Info("[pullImage] Image not cached, pulling")
	rc, err := node.Engine.ImagePull(ctx, image, false)
	defer utils.EnsureReaderClosed(ctx, rc)
	if err != nil {
		log.Errorf(ctx, "[pullImage] Error during pulling image %s: %v", image, err)
		return errors.WithStack(err)
	}
	log.Infof(ctx, "[pullImage] Done pulling image %s", image)
	return nil
}

func makeCopyMessage(id, name, path string, err error, data io.ReadCloser) *types.CopyMessage {
	return &types.CopyMessage{
		ID:    id,
		Name:  name,
		Path:  path,
		Error: err,
		Data:  data,
	}
}

func processVirtualizationInStream(
	ctx context.Context,
	inStream io.WriteCloser,
	inCh <-chan []byte,
	resizeFunc func(height, width uint) error,
) <-chan struct{} { // nolint
	specialPrefixCallback := map[string]func([]byte){
		string(winchCommand): func(body []byte) {
			w := &window{}
			if err := json.Unmarshal(body, w); err != nil {
				log.Errorf(ctx, "[processVirtualizationInStream] invalid winch command: %q", body)
				return
			}
			if err := resizeFunc(w.Height, w.Width); err != nil {
				log.Errorf(ctx, "[processVirtualizationInStream] resize window error: %v", err)
				return
			}
		},

		string(escapeCommand): func(_ []byte) {
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
	utils.SentryGo(func() {
		defer close(done)
		defer inStream.Close()

		for cmd := range inCh {
			if len(cmd) == 0 {
				continue
			}
			if f, ok := specialPrefixCallback[string(cmd[:1])]; ok {
				f(cmd[1:])
				continue
			}
			if _, err := inStream.Write(cmd); err != nil {
				log.Errorf(ctx, "[rawProcessVirtualizationInStream] failed to write virtual input stream: %v", err)
				continue
			}
		}
	})

	return done
}

func processVirtualizationOutStream(
	ctx context.Context,
	outStream io.ReadCloser,
	splitFunc bufio.SplitFunc,
	split byte,

) <-chan []byte {
	outCh := make(chan []byte)
	utils.SentryGo(func() {
		defer close(outCh)
		if outStream == nil {
			return
		}
		defer outStream.Close()
		scanner := bufio.NewScanner(outStream)
		scanner.Split(splitFunc)
		for scanner.Scan() {
			bs := scanner.Bytes()
			if split != 0 {
				bs = append(bs, split)
			}
			outCh <- bs
		}
		if err := scanner.Err(); err != nil {
			log.Warnf(ctx, "[processVirtualizationOutStream] failed to read output from output stream: %v", err)
		}
	})
	return outCh
}

func processBuildImageStream(ctx context.Context, reader io.ReadCloser) chan *types.BuildImageMessage {
	ch := make(chan *types.BuildImageMessage)
	utils.SentryGo(func() {
		defer close(ch)
		defer utils.EnsureReaderClosed(ctx, reader)
		decoder := json.NewDecoder(reader)
		for {
			message := &types.BuildImageMessage{}
			err := decoder.Decode(message)
			if err != nil {
				if err != io.EOF {
					malformed, _ := ioutil.ReadAll(decoder.Buffered()) // TODO err check
					log.Errorf(ctx, "[processBuildImageStream] Decode image message failed %v, buffered: %s", err, string(malformed))
					message.Error = err.Error()
					ch <- message
				}
				break
			}
			ch <- message
		}
	})
	return ch
}

func processStdStream(ctx context.Context, stdout, stderr io.ReadCloser, splitFunc bufio.SplitFunc, split byte) chan types.StdStreamMessage {
	ch := make(chan types.StdStreamMessage)

	wg := sync.WaitGroup{}

	wg.Add(1)
	utils.SentryGo(func() {
		defer wg.Done()
		for data := range processVirtualizationOutStream(ctx, stdout, splitFunc, split) {
			ch <- types.StdStreamMessage{Data: data, StdStreamType: types.Stdout}
		}
	})

	wg.Add(1)
	utils.SentryGo(func() {
		defer wg.Done()
		for data := range processVirtualizationOutStream(ctx, stderr, splitFunc, split) {
			ch <- types.StdStreamMessage{Data: data, StdStreamType: types.Stderr}
		}
	})

	utils.SentryGo(func() {
		defer close(ch)
		wg.Wait()
	})

	return ch
}
