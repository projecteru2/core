package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRawParams(t *testing.T) {
	var r RawParams

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
	}

	assert.Equal(t, r.Int64("int64"), int64(1))
	assert.Equal(t, r.Int64("str-int"), int64(1))
	assert.Equal(t, r.Int64("float-int"), int64(2))
	assert.Equal(t, r.Float64("float64"), 1.999999999999999999999)
	assert.Equal(t, r.String("string"), "string")
	assert.Equal(t, r.StringSlice("string-slice"), []string{"string", "string"})
	assert.Equal(t, r.OneOfStringSlice("?", "string-slice"), []string{"string", "string"})
	assert.Equal(t, r.Bool("bool"), true)
	assert.Equal(t, r.RawParams("raw-params")["int64"], 1)
	assert.Equal(t, r.IsSet("?"), false)
}
