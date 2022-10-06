package calcium

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"

	"github.com/pkg/errors"
	"github.com/projecteru2/core/engine"
	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

var winchCommand = []byte{0x80}  // 128, non-ASCII
var escapeCommand = []byte{0x1d} // 29, ^]

type window struct {
	Height uint `json:"Row"`
	Width  uint `json:"Col"`
}

func (c *Calcium) execuateInside(ctx context.Context, client engine.API, ID, cmd, user string, env []string, privileged bool) ([]byte, error) {
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

	for m := range c.processStdStream(ctx, stdout, stderr, bufio.ScanLines, byte('\n')) {
		b = append(b, m.Data...)
	}

	exitCode, err := client.ExecExitCode(ctx, ID, execID)
	if err != nil {
		return b, errors.WithStack(err)
	}
	if exitCode != 0 {
		return b, errors.WithStack(fmt.Errorf("%s", b))
	}
	return b, nil
}

func (c *Calcium) processVirtualizationInStream(
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
	return c.rawProcessVirtualizationInStream(ctx, inStream, inCh, specialPrefixCallback)
}

func (c *Calcium) rawProcessVirtualizationInStream(
	ctx context.Context,
	inStream io.WriteCloser,
	inCh <-chan []byte,
	specialPrefixCallback map[string]func([]byte),
) <-chan struct{} {
	done := make(chan struct{})
	_ = c.pool.Invoke(func() {
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

func (c *Calcium) processVirtualizationOutStream(
	ctx context.Context,
	outStream io.ReadCloser,
	splitFunc bufio.SplitFunc,
	split byte,

) <-chan []byte {
	outCh := make(chan []byte)
	_ = c.pool.Invoke(func() {
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

func (c *Calcium) processBuildImageStream(ctx context.Context, reader io.ReadCloser) chan *types.BuildImageMessage {
	ch := make(chan *types.BuildImageMessage)
	_ = c.pool.Invoke(func() {
		defer close(ch)
		defer utils.EnsureReaderClosed(ctx, reader)
		decoder := json.NewDecoder(reader)
		for {
			message := &types.BuildImageMessage{}
			err := decoder.Decode(message)
			if err != nil {
				if err != io.EOF {
					malformed, _ := io.ReadAll(decoder.Buffered()) // TODO err check
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

func (c *Calcium) processStdStream(ctx context.Context, stdout, stderr io.ReadCloser, splitFunc bufio.SplitFunc, split byte) chan types.StdStreamMessage {
	ch := make(chan types.StdStreamMessage)

	wg := sync.WaitGroup{}

	wg.Add(1)
	_ = c.pool.Invoke(func() {
		defer wg.Done()
		for data := range c.processVirtualizationOutStream(ctx, stdout, splitFunc, split) {
			ch <- types.StdStreamMessage{Data: data, StdStreamType: types.Stdout}
		}
	})

	wg.Add(1)
	_ = c.pool.Invoke(func() {
		defer wg.Done()
		for data := range c.processVirtualizationOutStream(ctx, stderr, splitFunc, split) {
			ch <- types.StdStreamMessage{Data: data, StdStreamType: types.Stderr}
		}
	})

	_ = c.pool.Invoke(func() {
		defer close(ch)
		wg.Wait()
	})

	return ch
}
