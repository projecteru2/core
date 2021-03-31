package wal

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/projecteru2/core/wal/kv"
)

func TestLogFailedAsNoSuchHandler(t *testing.T) {
	hydro := NewHydro()
	commit, err := hydro.Log("create", struct{}{})
	require.Error(t, err)
	require.Nil(t, commit)
}

func TestLogFailedAsEncodeError(t *testing.T) {
	var checked, handled, encoded, decoded bool
	eventype := "create"
	handler := newTestEventHandler(eventype, &checked, &handled, &encoded, &decoded)
	handler.encode = func(interface{}) ([]byte, error) { return nil, fmt.Errorf("encode error") }

	hydro := NewHydro()
	hydro.kv = kv.NewMockedKV()
	hydro.Register(handler)

	commit, err := hydro.Log(eventype, struct{}{})
	require.Error(t, err)
	require.Nil(t, commit)
	require.False(t, encoded)
	require.False(t, checked)
	require.False(t, decoded)
	require.False(t, handled)
}

func TestLogWithCommitEvent(t *testing.T) {
	var checked, handled, encoded, decoded bool
	eventype := "create"
	handler := newTestEventHandler(eventype, &checked, &handled, &encoded, &decoded)

	hydro := NewHydro()
	hydro.kv = kv.NewMockedKV()
	hydro.Register(handler)

	commit, err := hydro.Log(eventype, struct{}{})
	require.NoError(t, err)
	require.NotNil(t, commit)

	require.NoError(t, commit())
	require.True(t, encoded)
	require.False(t, decoded)
	require.False(t, checked)
	require.False(t, handled)
}

func TestRecoverFailedAsNoSuchHandler(t *testing.T) {
	var checked, handled, encoded, decoded bool
	eventype := "create"
	handler := newTestEventHandler(eventype, &checked, &handled, &encoded, &decoded)

	hydro := NewHydro()
	hydro.kv = kv.NewMockedKV()
	hydro.Register(handler)

	commit, err := hydro.Log(eventype, struct{}{})
	require.NoError(t, err)
	require.NotNil(t, commit)

	hydro.handlers.Delete(eventype)

	hydro.Recover()
	require.True(t, encoded)
	require.False(t, decoded)
	require.False(t, checked)
	require.False(t, handled)
}

func TestRecoverFailedAsCheckError(t *testing.T) {
	var checked, handled, encoded, decoded bool
	eventype := "create"
	handler := newTestEventHandler(eventype, &checked, &handled, &encoded, &decoded)
	handler.check = func(interface{}) (bool, error) {
		checked = true
		return false, fmt.Errorf("check error")
	}

	hydro := NewHydro()
	hydro.kv = kv.NewMockedKV()
	hydro.Register(handler)

	commit, err := hydro.Log(eventype, struct{}{})
	require.NoError(t, err)
	require.NotNil(t, commit)

	hydro.Recover()
	require.True(t, encoded)
	require.True(t, decoded)
	require.True(t, checked)
	require.False(t, handled)
}

func TestDecodeEventFailedAsDecodeEntryError(t *testing.T) {
	hydro := NewHydro()
	ent := kv.MockedScanEntry{Value: []byte("x")}
	_, err := hydro.decodeEvent(ent)
	require.Error(t, err)
}

func TestDecodeEventFailedAsInvalidEventID(t *testing.T) {
	hydro := NewHydro()
	ent := kv.MockedScanEntry{Key: "/events/x", Value: []byte("{}")}
	_, err := hydro.decodeEvent(ent)
	require.Error(t, err)
}

func TestDecodeEventFailedAsEntryError(t *testing.T) {
	hydro := NewHydro()
	expErr := fmt.Errorf("expects an error")
	ent := kv.MockedScanEntry{Err: expErr}
	_, err := hydro.decodeEvent(ent)
	require.Error(t, err)
	require.Equal(t, expErr.Error(), err.Error())
}

func TestRecoverFailedAsDecodeLogError(t *testing.T) {
	var checked, handled, encoded, decoded bool
	eventype := "create"
	handler := newTestEventHandler(eventype, &checked, &handled, &encoded, &decoded)
	handler.decode = func([]byte) (interface{}, error) {
		decoded = true
		return nil, fmt.Errorf("decode error")
	}

	hydro := NewHydro()
	hydro.kv = kv.NewMockedKV()
	hydro.Register(handler)

	commit, err := hydro.Log(eventype, struct{}{})
	require.NoError(t, err)
	require.NotNil(t, commit)

	hydro.Recover()
	require.True(t, encoded)
	require.True(t, decoded)
	require.False(t, checked)
	require.False(t, handled)
}

func TestHydroRecoverDiscardNoNeedEvent(t *testing.T) {
	var checked, handled, encoded, decoded bool
	check := func(interface{}) (need bool, err error) {
		checked = true
		return
	}

	eventype := "create"
	handler := newTestEventHandler(eventype, &checked, &handled, &encoded, &decoded)
	handler.check = check

	hydro := NewHydro()
	hydro.kv = kv.NewMockedKV()
	hydro.Register(handler)

	commit, err := hydro.Log(eventype, struct{}{})
	require.NoError(t, err)
	require.NotNil(t, commit)

	hydro.Recover()
	require.True(t, encoded)
	require.True(t, decoded)
	require.True(t, checked)
	require.False(t, handled)
}

func TestHydroRecover(t *testing.T) {
	var checked, handled, encoded, decoded bool
	eventype := "create"
	handler := newTestEventHandler(eventype, &checked, &handled, &encoded, &decoded)

	hydro := NewHydro()
	hydro.kv = kv.NewMockedKV()
	hydro.Register(handler)

	commit, err := hydro.Log(eventype, struct{}{})
	require.NoError(t, err)
	require.NotNil(t, commit)

	hydro.Recover()
	require.True(t, encoded)
	require.True(t, decoded)
	require.True(t, checked)
	require.True(t, handled)

	// The handled events should be removed.
	ch, _ := hydro.kv.Scan([]byte(EventPrefix))
	for range ch {
		require.Fail(t, "the events should be deleted")
	}
}

func TestHydroEventKeyMustPadZero(t *testing.T) {
	event := HydroEvent{ID: 15}
	require.Equal(t, "/events/000000000000000f", string(event.Key()))
}

func TestHydroEventParseIDShouldRemovePadding(t *testing.T) {
	id, err := parseHydroEventID([]byte("/events/00000000000000000000000000f"))
	require.NoError(t, err)
	require.Equal(t, uint64(15), id)
}

func TestHydroRecoverWithRealLithium(t *testing.T) {
	dir, rmdir := tempdir(t)
	defer rmdir()

	hydro := NewHydro()
	// Uses a real Lithium instance rather than a mocked one.
	require.NoError(t, hydro.Open(filepath.Join(dir, "temp.wal"), time.Second))

	handler := SimpleEventHandler{
		event:  "create",
		encode: func(interface{}) ([]byte, error) { return []byte("{}"), nil },
		decode: func([]byte) (interface{}, error) { return struct{}{}, nil },
		check:  func(interface{}) (bool, error) { return true, nil },
		handle: func(interface{}) error { return nil },
	}
	hydro.Register(handler)

	hydro.Log(handler.event, struct{}{})
	hydro.Log(handler.event, struct{}{})
	hydro.Log(handler.event, struct{}{})

	hydro.Recover()

	ch, _ := hydro.kv.Scan([]byte(EventPrefix))
	for range ch {
		require.FailNow(t, "expects no data")
	}
}

func tempdir(t *testing.T) (string, func()) {
	dir, err := ioutil.TempDir("", "temp.wal")
	require.NoError(t, err)
	return dir, func() { os.RemoveAll(dir) }
}

func newTestEventHandler(eventype string, checked, handled, encoded, decoded *bool) SimpleEventHandler {
	check := func(interface{}) (bool, error) {
		*checked = true
		return true, nil
	}

	handle := func(interface{}) (err error) {
		*handled = true
		return
	}

	encode := func(interface{}) (bs []byte, err error) {
		*encoded = true
		return
	}

	decode := func([]byte) (item interface{}, err error) {
		*decoded = true
		return
	}

	return SimpleEventHandler{
		event:  eventype,
		encode: encode,
		decode: decode,
		check:  check,
		handle: handle,
	}
}
