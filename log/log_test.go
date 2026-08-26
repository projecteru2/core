package log

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"gopkg.in/natefinch/lumberjack.v2"

	"github.com/projecteru2/core/types"
)

func TestVerblessFormatIsNotPadded(t *testing.T) {
	assert.Equal(t, `{"level":"info","k":"v","message":"server started"}`, capture(t, func() {
		WithField("k", "v").Infof(t.Context(), "server started")
	}))
}

func TestEveryArgIsRendered(t *testing.T) {
	assert.Equal(t, `{"level":"info","k":"v","message":"a b"}`, capture(t, func() {
		WithField("k", "v").Info(t.Context(), "a", "b")
	}))
	assert.Equal(t, `{"level":"debug","k":"v","message":"a b"}`, capture(t, func() {
		WithField("k", "v").Debug(t.Context(), "a", "b")
	}))
}

func TestConsoleLogsGoToStderr(t *testing.T) {
	w := logWriter(&types.ServerLogConfig{})
	cw, ok := w.(zerolog.ConsoleWriter)
	assert.True(t, ok)
	assert.Equal(t, os.Stderr, cw.Out)

	assert.Equal(t, os.Stderr, logWriter(&types.ServerLogConfig{UseJSON: true}))
}

func TestFileLogsStillGoToTheFile(t *testing.T) {
	name := filepath.Join(t.TempDir(), "core.log")
	w := logWriter(&types.ServerLogConfig{Filename: name})
	lj, ok := w.(*lumberjack.Logger)
	assert.True(t, ok)
	assert.Equal(t, name, lj.Filename)
}

func capture(t *testing.T, f func()) string {
	t.Helper()
	var buf bytes.Buffer
	globalLogger = zerolog.New(&buf).Level(zerolog.DebugLevel)
	f()
	line := buf.String()
	if i := len(line); i > 0 && line[i-1] == '\n' {
		line = line[:i-1]
	}
	return line
}
