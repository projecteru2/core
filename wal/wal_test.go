package wal

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/projecteru2/core/wal/kv"
)

func TestRecover(t *testing.T) {
	var checked bool
	check := func(interface{}) (bool, error) {
		checked = true
		return true, nil
	}

	var handled bool
	handle := func(interface{}) (err error) {
		handled = true
		return
	}

	var encoded bool
	encode := func(interface{}) (bs []byte, err error) {
		encoded = true
		return
	}

	var decoded bool
	decode := func([]byte) (item interface{}, err error) {
		decoded = true
		return
	}

	path := "/tmp/wal.unitest.wal"
	os.Remove(path)

	var wal WAL
	var err error
	wal, err = NewHydro(path, time.Second)
	assert.NoError(t, err)
	defer wal.Close()

	hydro, ok := wal.(*Hydro)
	assert.True(t, ok)
	assert.NotNil(t, hydro)
	hydro.stor = kv.NewMockedKV()

	eventype := "create"

	wal.Register(simpleEventHandler{
		event:  eventype,
		encode: encode,
		decode: decode,
		check:  check,
		handle: handle,
	})

	wal.Log(eventype, struct{}{})

	wal.Recover(context.TODO())
	assert.True(t, checked)
	assert.True(t, handled)
	assert.True(t, encoded)
	assert.True(t, decoded)
}

// simpleEventHandler simply implements the EventHandler.
type simpleEventHandler struct {
	event  string
	check  func(raw interface{}) (bool, error)
	encode func(interface{}) ([]byte, error)
	decode func([]byte) (interface{}, error)
	handle func(interface{}) error
}

// Event .
func (h simpleEventHandler) Typ() string {
	return h.event
}

// Check .
func (h simpleEventHandler) Check(ctx context.Context, raw interface{}) (bool, error) {
	return h.check(raw)
}

// Encode .
func (h simpleEventHandler) Encode(raw interface{}) ([]byte, error) {
	return h.encode(raw)
}

// Decode .
func (h simpleEventHandler) Decode(bs []byte) (interface{}, error) {
	return h.decode(bs)
}

// Handle .
func (h simpleEventHandler) Handle(ctx context.Context, raw interface{}) error {
	return h.handle(raw)
}
