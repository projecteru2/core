package wal

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

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

	require.NoError(t, Open(path, time.Second))
	defer Close()

	hydro, ok := wal.(*Hydro)
	require.True(t, ok)
	require.NotNil(t, hydro)
	hydro.kv = kv.NewMockedKV()

	eventype := "create"

	Register(SimpleEventHandler{
		event:  eventype,
		encode: encode,
		decode: decode,
		check:  check,
		handle: handle,
	})

	Log(eventype, struct{}{})

	Recover()
	require.True(t, checked)
	require.True(t, handled)
	require.True(t, encoded)
	require.True(t, decoded)
}
