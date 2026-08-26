package types

import "testing"

func TestCacheKeyIsStableAndDistinct(t *testing.T) {
	a := NewParams("n1", "cocoon://1.2.3.4")
	b := NewParams("n2", "cocoon://1.2.3.4")
	c := NewParams("n1", "cocoon://5.6.7.8")

	if a.CacheKey() != a.CacheKey() {
		t.Error("CacheKey is not stable across calls")
	}
	if a.CacheKey() != b.CacheKey() {
		t.Error("CacheKey must not depend on the nodename")
	}
	if a.CacheKey() == c.CacheKey() {
		t.Error("CacheKey must change with the endpoint")
	}
}
