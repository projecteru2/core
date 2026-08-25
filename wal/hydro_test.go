package wal

import (
	"context"
	"fmt"
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/projecteru2/core/wal/kv"
)

func TestLogFailedAsNoSuchHandler(t *testing.T) {
	hydro, _ := NewHydro(path.Join(t.TempDir(), "1"), time.Second)
	commit, err := hydro.Log("create", struct{}{})
	assert.Error(t, err)
	assert.Nil(t, commit)
}

func TestLogFailedAsEncodeError(t *testing.T) {
	var handled, encoded, decoded bool
	eventype := "create"
	handler := newTestEventHandler(eventype, &handled, &encoded, &decoded)
	handler.encode = func(any) ([]byte, error) { return nil, fmt.Errorf("encode error") }

	hydro, _ := NewHydro(path.Join(t.TempDir(), "1"), time.Second)
	hydro.store = kv.NewMockedKV()
	hydro.Register(handler)

	commit, err := hydro.Log(eventype, struct{}{})
	assert.Error(t, err)
	assert.Nil(t, commit)
	assert.False(t, encoded)
	assert.False(t, decoded)
	assert.False(t, handled)
}

func TestLogWithCommitEvent(t *testing.T) {
	var handled, encoded, decoded bool
	eventype := "create"
	handler := newTestEventHandler(eventype, &handled, &encoded, &decoded)

	hydro, _ := NewHydro(path.Join(t.TempDir(), "1"), time.Second)
	hydro.store = kv.NewMockedKV()
	hydro.Register(handler)

	commit, err := hydro.Log(eventype, struct{}{})
	assert.NoError(t, err)
	assert.NotNil(t, commit)

	assert.NoError(t, commit())
	assert.True(t, encoded)
	assert.False(t, decoded)
	assert.False(t, handled)
}

func TestRecoverFailedAsNoSuchHandler(t *testing.T) {
	var handled, encoded, decoded bool
	eventype := "create"
	handler := newTestEventHandler(eventype, &handled, &encoded, &decoded)

	hydro, _ := NewHydro(path.Join(t.TempDir(), "1"), time.Second)
	hydro.store = kv.NewMockedKV()
	hydro.Register(handler)

	commit, err := hydro.Log(eventype, struct{}{})
	assert.NoError(t, err)
	assert.NotNil(t, commit)

	hydro.handlers.Delete(eventype)

	hydro.Recover(context.Background())
	assert.True(t, encoded)
	assert.False(t, decoded)
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

func TestRecoverStopsOnScanError(t *testing.T) {
	var handled, encoded, decoded bool
	handler := newTestEventHandler("create", &handled, &encoded, &decoded)

	hydro, _ := NewHydro(path.Join(t.TempDir(), "1"), time.Second)
	hydro.store = scanErrorKV{MockedKV: kv.NewMockedKV()}
	hydro.Register(handler)

	commit, err := hydro.Log("create", struct{}{})
	assert.NoError(t, err)
	assert.NotNil(t, commit)

	hydro.Recover(context.Background())
	assert.False(t, decoded)
	assert.False(t, handled)
}

func TestRecoverFailedAsDecodeLogError(t *testing.T) {
	var handled, encoded, decoded bool
	eventype := "create"
	handler := newTestEventHandler(eventype, &handled, &encoded, &decoded)
	handler.decode = func([]byte) (any, error) {
		decoded = true
		return nil, fmt.Errorf("decode error")
	}

	hydro, _ := NewHydro(path.Join(t.TempDir(), "1"), time.Second)
	hydro.store = kv.NewMockedKV()
	hydro.Register(handler)

	commit, err := hydro.Log(eventype, struct{}{})
	assert.NoError(t, err)
	assert.NotNil(t, commit)

	hydro.Recover(context.Background())
	assert.True(t, encoded)
	assert.True(t, decoded)
	assert.False(t, handled)
}

func TestHydroRecover(t *testing.T) {
	var handled, encoded, decoded bool
	eventype := "create"
	handler := newTestEventHandler(eventype, &handled, &encoded, &decoded)

	hydro, _ := NewHydro(path.Join(t.TempDir(), "1"), time.Second)
	hydro.store = kv.NewMockedKV()
	hydro.Register(handler)

	commit, err := hydro.Log(eventype, struct{}{})
	assert.NoError(t, err)
	assert.NotNil(t, commit)

	hydro.Recover(context.Background())
	assert.True(t, encoded)
	assert.True(t, decoded)
	assert.True(t, handled)

	ch, _ := hydro.store.Scan([]byte(eventPrefix))
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
		encode: func(any) ([]byte, error) { return []byte("{}"), nil },
		decode: func([]byte) (any, error) { return struct{}{}, nil },
		handle: func(any) error { return nil },
	}
	hydro.Register(handler)

	hydro.Log(handler.event, struct{}{})
	hydro.Log(handler.event, struct{}{})
	hydro.Log(handler.event, struct{}{})

	hydro.Recover(context.Background())

	ch, _ := hydro.store.Scan([]byte(eventPrefix))
	for range ch {
		assert.FailNow(t, "expects no data")
	}
}

func newTestEventHandler(eventype string, handled, encoded, decoded *bool) simpleEventHandler {
	handle := func(any) (err error) {
		*handled = true
		return err
	}

	encode := func(any) (bs []byte, err error) {
		*encoded = true
		return bs, err
	}

	decode := func([]byte) (item any, err error) {
		*decoded = true
		return item, err
	}

	return simpleEventHandler{
		event:  eventype,
		encode: encode,
		decode: decode,
		handle: handle,
	}
}

type scanErrorKV struct {
	*kv.MockedKV
}

func (k scanErrorKV) Scan(prefix []byte) (<-chan kv.ScanEntry, func()) {
	scanned, _ := k.MockedKV.Scan(prefix)
	entries := []kv.ScanEntry{kv.MockedScanEntry{Err: fmt.Errorf("scan error")}}
	for entry := range scanned {
		entries = append(entries, entry)
	}

	ch := make(chan kv.ScanEntry, len(entries))
	for _, entry := range entries {
		ch <- entry
	}
	close(ch)

	return ch, func() {}
}
