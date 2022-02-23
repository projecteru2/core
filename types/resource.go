package types

import (
	"fmt"
	"strconv"
)

// ResourceMeta for messages and workload to store
type ResourceMeta map[string]WorkloadResourceArgs

// RawParams .
type RawParams map[string]interface{}

// IsSet .
func (r RawParams) IsSet(key string) bool {
	_, ok := r[key]
	return ok
}

// Float64 .
func (r RawParams) Float64(key string) float64 {
	res, _ := strconv.ParseFloat(fmt.Sprintf("%v", r[key]), 64)
	return res
}

// Int64 .
func (r RawParams) Int64(key string) int64 {
	res, _ := strconv.ParseInt(fmt.Sprintf("%v", r[key]), 10, 64)
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
			} else {
				return nil
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
	return r.IsSet(key)
}

// NodeResourceOpts .
type NodeResourceOpts RawParams

// NodeResourceArgs .
type NodeResourceArgs RawParams

// WorkloadResourceOpts .
type WorkloadResourceOpts RawParams

// WorkloadResourceArgs .
type WorkloadResourceArgs RawParams

// EngineArgs .
type EngineArgs RawParams
