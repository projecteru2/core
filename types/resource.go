package types

import (
	"fmt"
	"strconv"

	"github.com/mitchellh/mapstructure"
)

// RawParams .
type RawParams map[string]interface{}

// IsSet .
func (r RawParams) IsSet(key string) bool {
	_, ok := r[key]
	return ok
}

// Float64 .
func (r RawParams) Float64(key string) float64 {
	if !r.IsSet(key) {
		return float64(0.0)
	}
	res, _ := strconv.ParseFloat(fmt.Sprintf("%+v", r[key]), 64)
	return res
}

// Int64 .
func (r RawParams) Int64(key string) int64 {
	if !r.IsSet(key) {
		return int64(0)
	}
	var str string
	if f, ok := r[key].(float64); ok {
		str = fmt.Sprintf("%.0f", f)
	} else {
		str = fmt.Sprintf("%+v", r[key])
	}
	res, _ := strconv.ParseInt(str, 10, 64)
	return res
}

// String .
func (r RawParams) String(key string) string {
	if !r.IsSet(key) {
		return ""
	}
	if str, ok := r[key].(string); ok {
		return str
	}
	return ""
}

// StringSlice .
func (r RawParams) StringSlice(key string) []string {
	return sliceHelper[string](r, key)
}

// OneOfStringSlice .
func (r RawParams) OneOfStringSlice(keys ...string) []string {
	for _, key := range keys {
		if res := r.StringSlice(key); len(res) > 0 {
			return res
		}
	}
	return nil
}

// Bool .
func (r RawParams) Bool(key string) bool {
	s := r.IsSet(key)
	b, ok := r[key].(bool)
	return (!ok && s) || (ok && s && b)
}

// RawParams .
func (r RawParams) RawParams(key string) *RawParams {
	var n *RawParams
	if r.IsSet(key) {
		if m, ok := r[key].(map[string]interface{}); ok {
			n = &RawParams{}
			_ = mapstructure.Decode(m, n)
		}
	}
	return n
}

// SliceRawParams .
func (r RawParams) SliceRawParams(key string) []*RawParams {
	res := sliceHelper[map[string]interface{}](r, key)
	if res == nil {
		return nil
	}
	n := make([]*RawParams, len(res))
	for i, v := range res {
		_ = mapstructure.Decode(v, &n[i])
	}
	return n
}

func sliceHelper[T any](r RawParams, key string) []T {
	if !r.IsSet(key) {
		return nil
	}
	if s, ok := r[key].([]T); ok {
		return s
	}
	var res []T
	if s, ok := r[key].([]interface{}); ok {
		res = make([]T, len(s))
		for i, v := range s {
			if r, ok := v.(T); ok {
				res[i] = r
			} else {
				return nil
			}
		}
	}
	return res
}

// Resources all cosmos use this
type Resources map[string]*RawParams
