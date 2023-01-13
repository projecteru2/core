package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRawParams(t *testing.T) {
	var r RawParams

	r = RawParams{
		"cde": 1,
		"bef": []interface{}{1, 2, 3, "1"},
		"efg": []string{},
	}
	assert.Equal(t, r.Float64("abc"), 0.0)
	assert.Equal(t, r.Int64("abc"), int64(0))
	assert.Equal(t, r.String("abc"), "")
	assert.Equal(t, r.String("cde"), "")
	assert.Len(t, r.StringSlice("bef"), 1)
	assert.Nil(t, r.OneOfStringSlice("efg"))
	assert.Len(t, *r.RawParams("fgd"), 0)

	r = RawParams{
		"int64":        1,
		"str-int":      "1",
		"float-int":    1.999999999999999999999,
		"float64":      1.999999999999999999999,
		"string":       "string",
		"string-slice": []string{"string", "string"},
		"bool":         nil,
		"raw-params": map[string]interface{}{
			"int64":        1,
			"str-int":      "1",
			"float-int":    1.999999999999999999999,
			"float64":      1.999999999999999999999,
			"string":       "string",
			"string-slice": []string{"string", "string"},
			"bool":         nil,
		},
		"slice-raw-params": []map[string]interface{}{
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
	assert.Equal(t, r.OneOfStringSlice("?", "string-slice"), []string{"string", "string"})
	assert.Equal(t, r.Bool("bool"), true)
	assert.Equal(t, (*r.RawParams("raw-params"))["int64"], 1)
	assert.Equal(t, (*r.SliceRawParams("slice-raw-params")[0])["int"], 1)
	assert.Equal(t, r.IsSet("?"), false)
}
