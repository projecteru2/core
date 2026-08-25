package types

import (
	"fmt"
	"strconv"

	"github.com/go-viper/mapstructure/v2"
)

type RawParams map[string]any

func (r RawParams) IsSet(key string) bool {
	_, ok := r[key]
	return ok
}

func (r RawParams) Float64(key string) float64 {
	if !r.IsSet(key) {
		return float64(0.0)
	}
	res, _ := strconv.ParseFloat(fmt.Sprintf("%+v", r[key]), 64)
	return res
}

func (r RawParams) Int64(key string) int64 {
	return intHelper[int64](r, key)
}

func (r RawParams) Int(key string) int {
	return intHelper[int](r, key)
}

func (r RawParams) String(key string) string {
	if !r.IsSet(key) {
		return ""
	}
	if str, ok := r[key].(string); ok {
		return str
	}
	return ""
}

func (r RawParams) StringSlice(key string) []string {
	return sliceHelper[string](r, key)
}

func (r RawParams) OneOfStringSlice(keys ...string) []string {
	for _, key := range keys {
		if res := r.StringSlice(key); len(res) > 0 {
			return res
		}
	}
	return nil
}

func (r RawParams) Bool(key string) bool {
	s := r.IsSet(key)
	b, ok := r[key].(bool)
	return (!ok && s) || (ok && s && b)
}

func (r RawParams) RawParams(key string) RawParams {
	var n RawParams
	if r.IsSet(key) {
		if m, ok := r[key].(map[string]any); ok {
			n = RawParams{}
			_ = mapstructure.Decode(m, &n)
		}
	}
	return n
}

func (r RawParams) SliceRawParams(key string) []RawParams {
	res := sliceHelper[map[string]any](r, key)
	if res == nil {
		return nil
	}
	n := make([]RawParams, len(res))
	for i, v := range res {
		_ = mapstructure.Decode(v, &n[i])
	}
	return n
}

// Resources maps a plugin name to its raw params.
type Resources map[string]RawParams

func sliceHelper[T any](r RawParams, key string) []T {
	if !r.IsSet(key) {
		return nil
	}
	if s, ok := r[key].([]T); ok {
		return s
	}
	var res []T
	if s, ok := r[key].([]any); ok {
		res = []T{}
		for _, v := range s {
			if r, ok := v.(T); ok {
				res = append(res, r)
			}
		}
	}
	return res
}

type integer interface{ int | int64 }

func intHelper[T integer](r RawParams, key string) T {
	if !r.IsSet(key) {
		return T(0)
	}
	var str string
	if f, ok := r[key].(float64); ok {
		str = fmt.Sprintf("%.0f", f)
	} else {
		str = fmt.Sprintf("%+v", r[key])
	}
	res, _ := strconv.ParseInt(str, 10, 64)
	return T(res)
}
