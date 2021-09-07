package wal

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/projecteru2/core/wal/kv"

	"github.com/stretchr/testify/require"
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

	var wal WAL = NewHydro()
	require.NoError(t, wal.Open(path, time.Second))
	defer wal.Close()

	hydro, ok := wal.(*Hydro)
	require.True(t, ok)
	require.NotNil(t, hydro)
	hydro.kv = kv.NewMockedKV()

	eventype := "create"

	wal.Register(SimpleEventHandler{
		event:  eventype,
		encode: encode,
		decode: decode,
		check:  check,
		handle: handle,
	})

	wal.Log(eventype, struct{}{})

	wal.Recover(context.TODO())
	require.True(t, checked)
	require.True(t, handled)
	require.True(t, encoded)
	require.True(t, decoded)
}
