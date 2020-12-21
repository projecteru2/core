package wal

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/projecteru2/core/wal/kv"
)

func TestLogFailedAsNoSuchHandler(t *testing.T) {
	hydro := NewHydro()
	commit, err := hydro.Log(context.Background(), "create", struct{}{})
	require.Error(t, err)
	require.Nil(t, commit)
}

func TestLogFailedAsEncodeError(t *testing.T) {
	var checked, handled, encoded, decoded bool
	eventype := "create"
	handler := newTestEventHandler(eventype, &checked, &handled, &encoded, &decoded)
	handler.Encode = func(interface{}) ([]byte, error) { return nil, fmt.Errorf("encode error") }

	hydro := NewHydro()
	hydro.kv = kv.NewMockedKV()
	hydro.Register(handler)

	commit, err := hydro.Log(context.Background(), eventype, struct{}{})
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

	commit, err := hydro.Log(context.Background(), eventype, struct{}{})
	require.NoError(t, err)
	require.NotNil(t, commit)

	require.NoError(t, commit(context.Background()))
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

	commit, err := hydro.Log(context.Background(), eventype, struct{}{})
	require.NoError(t, err)
	require.NotNil(t, commit)

	hydro.handlers.Delete(eventype)

	hydro.Recover(context.Background())
	require.True(t, encoded)
	require.False(t, decoded)
	require.False(t, checked)
	require.False(t, handled)
}

func TestRecoverFailedAsCheckError(t *testing.T) {
	var checked, handled, encoded, decoded bool
	eventype := "create"
	handler := newTestEventHandler(eventype, &checked, &handled, &encoded, &decoded)
	handler.Check = func(interface{}) (bool, error) {
		checked = true
		return false, fmt.Errorf("check error")
	}

	hydro := NewHydro()
	hydro.kv = kv.NewMockedKV()
	hydro.Register(handler)

	commit, err := hydro.Log(context.Background(), eventype, struct{}{})
	require.NoError(t, err)
	require.NotNil(t, commit)

	hydro.Recover(context.Background())
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
	handler.Decode = func([]byte) (interface{}, error) {
		decoded = true
		return nil, fmt.Errorf("decode error")
	}

	hydro := NewHydro()
	hydro.kv = kv.NewMockedKV()
	hydro.Register(handler)

	commit, err := hydro.Log(context.Background(), eventype, struct{}{})
	require.NoError(t, err)
	require.NotNil(t, commit)

	hydro.Recover(context.Background())
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
	handler.Check = check

	hydro := NewHydro()
	hydro.kv = kv.NewMockedKV()
	hydro.Register(handler)

	commit, err := hydro.Log(context.Background(), eventype, struct{}{})
	require.NoError(t, err)
	require.NotNil(t, commit)

	hydro.Recover(context.Background())
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

	commit, err := hydro.Log(context.Background(), eventype, struct{}{})
	require.NoError(t, err)
	require.NotNil(t, commit)

	hydro.Recover(context.Background())
	require.True(t, encoded)
	require.True(t, decoded)
	require.True(t, checked)
	require.True(t, handled)
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

func newTestEventHandler(eventype string, checked, handled, encoded, decoded *bool) EventHandler {
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

	return EventHandler{
		Event:  eventype,
		Encode: encode,
		Decode: decode,
		Check:  check,
		Handle: handle,
	}
}
