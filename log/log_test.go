package log

import (
	"bytes"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

func TestVerblessFormatIsNotPadded(t *testing.T) {
	assert.Equal(t, `{"level":"info","message":"server started"}`, capture(t, func() {
		Infof(t.Context(), "server started")
	}))
}

func TestEveryArgIsRendered(t *testing.T) {
	assert.Equal(t, `{"level":"info","message":"a b"}`, capture(t, func() {
		Info(t.Context(), "a", "b")
	}))
	assert.Equal(t, `{"level":"debug","k":"v","message":"a b"}`, capture(t, func() {
		WithField("k", "v").Debug(t.Context(), "a", "b")
	}))
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
