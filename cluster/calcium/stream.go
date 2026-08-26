package calcium

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"slices"
	"sync"

	"github.com/cockroachdb/errors"

	"github.com/projecteru2/core/engine"
	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

var (
	winchCommand  = []byte{0x80} // 128, non-ASCII
	escapeCommand = []byte{0x1d} // 29, ^]
)

type prefixHandler func([]byte)

type window struct {
	Height uint `json:"Row"`
	Width  uint `json:"Col"`
}

func (c *Calcium) executeInside(ctx context.Context, client engine.API, ID, cmd, user string, env []string, privileged bool) ([]byte, error) {
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
		return nil, err
	}

	for m := range c.processStdStream(ctx, stdout, stderr, bufio.ScanLines, byte('\n')) {
		b = append(b, m.Data...)
	}

	exitCode, err := client.ExecExitCode(ctx, ID, execID)
	if err != nil {
		return b, err
	}
	if exitCode != 0 {
		return b, errors.New(string(b))
	}
	return b, nil
}

func (c *Calcium) processVirtualizationInStream(
	ctx context.Context,
	inStream io.WriteCloser,
	inCh <-chan []byte,
	resizeFunc func(height, width uint) error,
) {
	logger := log.WithFunc("calcium.processVirtualizationInStream")
	specialPrefixCallback := map[string]prefixHandler{
		string(winchCommand): func(body []byte) {
			w := &window{}
			if err := json.Unmarshal(body, w); err != nil {
				logger.Errorf(ctx, err, "invalid winch command: %q", body)
				return
			}
			if err := resizeFunc(w.Height, w.Width); err != nil {
				logger.Error(ctx, err, "resize window error")
				return
			}
		},

		string(escapeCommand): func(_ []byte) {
			_ = inStream.Close()
		},
	}
	c.rawProcessVirtualizationInStream(ctx, inStream, inCh, specialPrefixCallback)
}

func (c *Calcium) rawProcessVirtualizationInStream(
	ctx context.Context,
	inStream io.WriteCloser,
	inCh <-chan []byte,
	specialPrefixCallback map[string]prefixHandler,
) {
	utils.SentryGo(func() {
		defer func() {
			_ = inStream.Close()
		}()

		for cmd := range inCh {
			if len(cmd) == 0 {
				continue
			}
			if f, ok := specialPrefixCallback[string(cmd[:1])]; ok {
				f(cmd[1:])
				continue
			}
			if _, err := inStream.Write(cmd); err != nil {
				log.WithFunc("calcium.rawProcessVirtualizationInStream").Error(ctx, err, "failed to write virtual input stream")
			}
		}
	})
}

func (c *Calcium) processVirtualizationOutStream(
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
		defer func() {
			_ = outStream.Close()
		}()
		scanner := bufio.NewScanner(outStream)
		scanner.Buffer(nil, c.config.GRPCConfig.MaxRecvMsgSize)
		scanner.Split(splitFunc)
		for scanner.Scan() {
			bs := slices.Clone(scanner.Bytes())
			if split != 0 {
				bs = append(bs, split)
			}
			outCh <- bs
		}
		if err := scanner.Err(); err != nil {
			log.WithFunc("calcium.processVirtualizationOutStream").Warnf(ctx, "failed to read output from output stream: %+v", err)
		}
	})
	return outCh
}

func (c *Calcium) processBuildImageStream(ctx context.Context, reader io.ReadCloser) chan *types.BuildImageMessage {
	ch := make(chan *types.BuildImageMessage)
	utils.SentryGo(func() {
		defer close(ch)
		defer utils.EnsureReaderClosed(ctx, reader)
		decoder := json.NewDecoder(reader)
		for {
			message := &types.BuildImageMessage{}
			err := decoder.Decode(message)
			if err != nil {
				if !errors.Is(err, io.EOF) {
					malformed, _ := io.ReadAll(decoder.Buffered())
					log.WithFunc("calcium.processBuildImageStream").Errorf(ctx, err, "decode image message failed, buffered: %s", string(malformed))
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

	for _, source := range []struct {
		stream io.ReadCloser
		typ    types.StdStreamType
	}{{stdout, types.Stdout}, {stderr, types.Stderr}} {
		wg.Go(func() {
			defer log.SentryDefer()
			for data := range c.processVirtualizationOutStream(ctx, source.stream, splitFunc, split) {
				ch <- types.StdStreamMessage{Data: data, StdStreamType: source.typ}
			}
		})
	}

	utils.SentryGo(func() {
		defer close(ch)
		wg.Wait()
	})

	return ch
}
