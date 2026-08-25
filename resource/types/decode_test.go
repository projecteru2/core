package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDecodeKeysAMapByJSONTag(t *testing.T) {
	out := RawParams{}
	assert.NoError(t, Decode(decodeTarget{Name: "n1", Size: 3}, &out))
	assert.Equal(t, RawParams{"name": "n1", "size": float64(3)}, out)
}

func TestDecodeReadsAMapIntoAStruct(t *testing.T) {
	out := &decodeTarget{}
	assert.NoError(t, Decode(map[string]any{"name": "n1", "size": int64(3)}, out))
	assert.Equal(t, &decodeTarget{Name: "n1", Size: 3}, out)
}

func TestDecodeLeavesFieldsTheInputOmits(t *testing.T) {
	out := &decodeTarget{Name: "kept", Size: 7}
	assert.NoError(t, Decode(map[string]any{"size": 9}, out))
	assert.Equal(t, &decodeTarget{Name: "kept", Size: 9}, out)
}

func TestDecodeRejectsAFractionalInteger(t *testing.T) {
	assert.Error(t, Decode(map[string]any{"size": 1.5}, &decodeTarget{}))
}

func TestDecodeRejectsAnUnmarshalableInput(t *testing.T) {
	assert.Error(t, Decode(map[string]any{"size": func() {}}, &decodeTarget{}))
}

func BenchmarkDecode(b *testing.B) {
	in := map[string]any{"name": "n1", "size": int64(3)}
	for b.Loop() {
		out := &decodeTarget{}
		if err := Decode(in, out); err != nil {
			b.Fatal(err)
		}
	}
}

type decodeTarget struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
}
