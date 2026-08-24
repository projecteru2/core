package common

import (
	"maps"
	"slices"
)

type Endpoints map[string]struct{}

func (e Endpoints) Add(endpoint string) (changed bool) {
	if _, ok := e[endpoint]; !ok {
		e[endpoint] = struct{}{}
		changed = true
	}
	return changed
}

func (e Endpoints) Remove(endpoint string) (changed bool) {
	if _, ok := e[endpoint]; ok {
		delete(e, endpoint)
		changed = true
	}
	return changed
}

func (e Endpoints) ToSlice() []string {
	return slices.Collect(maps.Keys(e))
}
