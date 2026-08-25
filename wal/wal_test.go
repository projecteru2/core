package wal

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/projecteru2/core/wal/kv"
)

func TestRecover(t *testing.T) {
	var handled bool
	handle := func(any) (err error) {
		handled = true
		return err
	}

	var encoded bool
	encode := func(any) (bs []byte, err error) {
		encoded = true
		return bs, err
	}

	var decoded bool
	decode := func([]byte) (item any, err error) {
		decoded = true
		return item, err
	}

	path := filepath.Join(t.TempDir(), "wal.wal")

	var wal WAL
	var err error
	wal, err = NewHydro(path, time.Second)
	assert.NoError(t, err)
	defer wal.Close()

	hydro, ok := wal.(*Hydro)
	assert.True(t, ok)
	assert.NotNil(t, hydro)
	hydro.store = kv.NewMockedKV()

	eventype := "create"

	wal.Register(simpleEventHandler{
		event:  eventype,
		encode: encode,
		decode: decode,
		handle: handle,
	})

	wal.Log(eventype, struct{}{})

	wal.Recover(context.Background())
	assert.True(t, handled)
	assert.True(t, encoded)
	assert.True(t, decoded)
}

type simpleEventHandler struct {
	event  string
	encode func(any) ([]byte, error)
	decode func([]byte) (any, error)
	handle func(any) error
}

func (h simpleEventHandler) Typ() string {
	return h.event
}

func (h simpleEventHandler) Encode(raw any) ([]byte, error) {
	return h.encode(raw)
}

func (h simpleEventHandler) Decode(bs []byte) (any, error) {
	return h.decode(bs)
}

func (h simpleEventHandler) Handle(ctx context.Context, raw any) error {
	return h.handle(raw)
}
