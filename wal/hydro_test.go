package wal

import (
	"context"
	"fmt"
	"path"
	"testing"
	"time"

	"github.com/projecteru2/core/wal/kv"

	"github.com/stretchr/testify/assert"
)

func TestLogFailedAsNoSuchHandler(t *testing.T) {
	hydro, _ := NewHydro(path.Join(t.TempDir(), "1"), time.Second)
	commit, err := hydro.Log("create", struct{}{})
	assert.Error(t, err)
	assert.Nil(t, commit)
}

func TestLogFailedAsEncodeError(t *testing.T) {
	var checked, handled, encoded, decoded bool
	eventype := "create"
	handler := newTestEventHandler(eventype, &checked, &handled, &encoded, &decoded)
	handler.encode = func(interface{}) ([]byte, error) { return nil, fmt.Errorf("encode error") }

	hydro, _ := NewHydro(path.Join(t.TempDir(), "1"), time.Second)
	hydro.stor = kv.NewMockedKV()
	hydro.Register(handler)

	commit, err := hydro.Log(eventype, struct{}{})
	assert.Error(t, err)
	assert.Nil(t, commit)
	assert.False(t, encoded)
	assert.False(t, checked)
	assert.False(t, decoded)
	assert.False(t, handled)
}

func TestLogWithCommitEvent(t *testing.T) {
	var checked, handled, encoded, decoded bool
	eventype := "create"
	handler := newTestEventHandler(eventype, &checked, &handled, &encoded, &decoded)

	hydro, _ := NewHydro(path.Join(t.TempDir(), "1"), time.Second)
	hydro.stor = kv.NewMockedKV()
	hydro.Register(handler)

	commit, err := hydro.Log(eventype, struct{}{})
	assert.NoError(t, err)
	assert.NotNil(t, commit)

	assert.NoError(t, commit())
	assert.True(t, encoded)
	assert.False(t, decoded)
	assert.False(t, checked)
	assert.False(t, handled)
}

func TestRecoverFailedAsNoSuchHandler(t *testing.T) {
	var checked, handled, encoded, decoded bool
	eventype := "create"
	handler := newTestEventHandler(eventype, &checked, &handled, &encoded, &decoded)

	hydro, _ := NewHydro(path.Join(t.TempDir(), "1"), time.Second)
	hydro.stor = kv.NewMockedKV()
	hydro.Register(handler)

	commit, err := hydro.Log(eventype, struct{}{})
	assert.NoError(t, err)
	assert.NotNil(t, commit)

	hydro.Del(eventype)

	hydro.Recover(context.TODO())
	assert.True(t, encoded)
	assert.False(t, decoded)
	assert.False(t, checked)
	assert.False(t, handled)
}

func TestRecoverFailedAsCheckError(t *testing.T) {
	var checked, handled, encoded, decoded bool
	eventype := "create"
	handler := newTestEventHandler(eventype, &checked, &handled, &encoded, &decoded)
	handler.check = func(interface{}) (bool, error) {
		checked = true
		return false, fmt.Errorf("check error")
	}

	hydro, _ := NewHydro(path.Join(t.TempDir(), "1"), time.Second)
	hydro.stor = kv.NewMockedKV()
	hydro.Register(handler)

	commit, err := hydro.Log(eventype, struct{}{})
	assert.NoError(t, err)
	assert.NotNil(t, commit)

	hydro.Recover(context.TODO())
	assert.True(t, encoded)
	assert.True(t, decoded)
	assert.True(t, checked)
	assert.False(t, handled)
}

func TestDecodeEventFailedAsDecodeEntryError(t *testing.T) {
	hydro, _ := NewHydro(path.Join(t.TempDir(), "1"), time.Second)
	ent := kv.MockedScanEntry{Value: []byte("x")}
	_, err := hydro.decodeEvent(ent)
	assert.Error(t, err)
}

func TestDecodeEventFailedAsInvalidEventID(t *testing.T) {
	hydro, _ := NewHydro(path.Join(t.TempDir(), "1"), time.Second)
	ent := kv.MockedScanEntry{Key: "/events/x", Value: []byte("{}")}
	_, err := hydro.decodeEvent(ent)
	assert.Error(t, err)
}

func TestDecodeEventFailedAsEntryError(t *testing.T) {
	hydro, _ := NewHydro(path.Join(t.TempDir(), "1"), time.Second)
	expErr := fmt.Errorf("expects an error")
	ent := kv.MockedScanEntry{Err: expErr}
	_, err := hydro.decodeEvent(ent)
	assert.Error(t, err)
	assert.Equal(t, expErr.Error(), err.Error())
}

func TestRecoverFailedAsDecodeLogError(t *testing.T) {
	var checked, handled, encoded, decoded bool
	eventype := "create"
	handler := newTestEventHandler(eventype, &checked, &handled, &encoded, &decoded)
	handler.decode = func([]byte) (interface{}, error) {
		decoded = true
		return nil, fmt.Errorf("decode error")
	}

	hydro, _ := NewHydro(path.Join(t.TempDir(), "1"), time.Second)
	hydro.stor = kv.NewMockedKV()
	hydro.Register(handler)

	commit, err := hydro.Log(eventype, struct{}{})
	assert.NoError(t, err)
	assert.NotNil(t, commit)

	hydro.Recover(context.TODO())
	assert.True(t, encoded)
	assert.True(t, decoded)
	assert.False(t, checked)
	assert.False(t, handled)
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

	hydro, _ := NewHydro(path.Join(t.TempDir(), "1"), time.Second)
	hydro.stor = kv.NewMockedKV()
	hydro.Register(handler)

	commit, err := hydro.Log(eventype, struct{}{})
	assert.NoError(t, err)
	assert.NotNil(t, commit)

	hydro.Recover(context.TODO())
	assert.True(t, encoded)
	assert.True(t, decoded)
	assert.True(t, checked)
	assert.False(t, handled)
}

func TestHydroRecover(t *testing.T) {
	var checked, handled, encoded, decoded bool
	eventype := "create"
	handler := newTestEventHandler(eventype, &checked, &handled, &encoded, &decoded)

	hydro, _ := NewHydro(path.Join(t.TempDir(), "1"), time.Second)
	hydro.stor = kv.NewMockedKV()
	hydro.Register(handler)

	commit, err := hydro.Log(eventype, struct{}{})
	assert.NoError(t, err)
	assert.NotNil(t, commit)

	hydro.Recover(context.TODO())
	assert.True(t, encoded)
	assert.True(t, decoded)
	assert.True(t, checked)
	assert.True(t, handled)

	// The handled events should be removed.
	ch, _ := hydro.stor.Scan([]byte(eventPrefix))
	for range ch {
		assert.Fail(t, "the events should be deleted")
	}
}

func TestHydroEventKeyMustPadZero(t *testing.T) {
	event := HydroEvent{ID: 15}
	assert.Equal(t, "/events/000000000000000f", string(event.Key()))
}

func TestHydroEventParseIDShouldRemovePadding(t *testing.T) {
	id, err := parseHydroEventID([]byte("/events/00000000000000000000000000f"))
	assert.NoError(t, err)
	assert.Equal(t, uint64(15), id)
}

func TestHydroRecoverWithRealLithium(t *testing.T) {
	p := path.Join(t.TempDir(), "temp.wal")
	hydro, err := NewHydro(p, time.Second)
	assert.NoError(t, err)

	handler := simpleEventHandler{
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

	hydro.Recover(context.TODO())

	ch, _ := hydro.stor.Scan([]byte(eventPrefix))
	for range ch {
		assert.FailNow(t, "expects no data")
	}
}

func newTestEventHandler(eventype string, checked, handled, encoded, decoded *bool) simpleEventHandler {
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

	return simpleEventHandler{
		event:  eventype,
		encode: encode,
		decode: decode,
		check:  check,
		handle: handle,
	}
}
