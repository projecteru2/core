package types

import (
	"fmt"
	"strconv"
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
	if !r.IsSet(key) {
		return nil
	}
	if s, ok := r[key].([]string); ok {
		return s
	}
	res := []string{}
	if s, ok := r[key].([]interface{}); ok {
		for _, v := range s {
			if str, ok := v.(string); ok {
				res = append(res, str)
			}
		}
	}
	return res
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
func (r RawParams) RawParams(key string) map[string]interface{} {
	if r.IsSet(key) {
		if m, ok := r[key].(map[string]interface{}); ok {
			return m
		}
	}
	return map[string]interface{}{}
}

// Resources all cosmos use this
type Resources map[string]*RawParams
