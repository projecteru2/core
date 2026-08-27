package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSizeInBytesTakesEitherForm(t *testing.T) {
	r := RawParams{
		"bytes":    int64(1073741824),
		"json-num": float64(2147483648),
		"human":    "1G",
		"delta":    int64(-1073741824),
		"neg-str":  "-1G",
		"junk":     "1Q",
	}
	for key, want := range map[string]int64{
		"bytes":    1073741824,
		"json-num": 2147483648,
		"human":    1073741824,
		"delta":    -1073741824,
		"neg-str":  -1073741824,
		"absent":   0,
	} {
		got, err := r.SizeInBytes(key)
		assert.NoError(t, err, key)
		assert.Equal(t, want, got, key)
	}
	_, err := r.SizeInBytes("junk")
	assert.Error(t, err)
}

func TestRawParams(t *testing.T) {
	var r RawParams

	r = RawParams{
		"cde": 1,
		"bef": []any{1, 2, 3, "1"},
		"efg": []string{},
	}
	assert.Equal(t, r.Float64("abc"), 0.0)
	assert.Equal(t, r.Int64("abc"), int64(0))
	assert.Equal(t, r.String("abc"), "")
	assert.Equal(t, r.String("cde"), "")
	assert.Len(t, r.StringSlice("bef"), 1)
	assert.Nil(t, r.RawParams("fgd"))

	r = RawParams{
		"int64":        1,
		"str-int":      "1",
		"float-int":    1.999999999999999999999,
		"float64":      1.999999999999999999999,
		"string":       "string",
		"string-slice": []string{"string", "string"},
		"bool":         nil,
		"raw-params": map[string]any{
			"int64":        1,
			"str-int":      "1",
			"float-int":    1.999999999999999999999,
			"float64":      1.999999999999999999999,
			"string":       "string",
			"string-slice": []string{"string", "string"},
			"bool":         nil,
		},
		"slice-raw-params": []map[string]any{
			{"int": 1},
			{"float": 1},
		},
	}

	assert.Equal(t, r.Int64("int64"), int64(1))
	assert.Equal(t, r.Int64("str-int"), int64(1))
	assert.Equal(t, r.Int64("float-int"), int64(2))
	assert.Equal(t, r.Float64("float64"), 1.999999999999999999999)
	assert.Equal(t, r.String("string"), "string")
	assert.Equal(t, r.StringSlice("string-slice"), []string{"string", "string"})
	assert.Equal(t, r.Bool("bool"), true)
	assert.Equal(t, r.RawParams("raw-params")["int64"], 1)
	assert.Equal(t, r.SliceRawParams("slice-raw-params")[0]["int"], 1)
	assert.Equal(t, r.IsSet("?"), false)
}
