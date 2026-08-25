package types

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"
)

var (
	benchCA   = strings.Repeat("ca-pem-line\n", 128)
	benchCert = strings.Repeat("cert-pem-line\n", 128)
	benchKey  = strings.Repeat("key-pem-line\n", 128)

	benchSink string
)

func TestCacheKeyIsStableAndDistinct(t *testing.T) {
	a := NewParams("n1", "tcp://1.2.3.4:2376", benchCA, benchCert, benchKey)
	b := NewParams("n2", "tcp://1.2.3.4:2376", benchCA, benchCert, benchKey)
	c := NewParams("n1", "tcp://1.2.3.4:2376", benchCA, benchCert, "other")

	if a.CacheKey() != a.CacheKey() {
		t.Error("CacheKey is not stable across calls")
	}
	if a.CacheKey() != b.CacheKey() {
		t.Error("CacheKey must not depend on the nodename")
	}
	if a.CacheKey() == c.CacheKey() {
		t.Error("CacheKey must change with the credentials")
	}
	if got, want := a.CacheKey(), EndpointCacheKey("tcp://1.2.3.4:2376", benchCA, benchCert, benchKey); got != want {
		t.Errorf("EndpointCacheKey diverged: got %q, want %q", got, want)
	}
}

func BenchmarkCacheKey(b *testing.B) {
	p := NewParams("n1", "tcp://1.2.3.4:2376", benchCA, benchCert, benchKey)
	for b.Loop() {
		benchSink = p.CacheKey()
	}
}

func BenchmarkCacheKeyLegacy(b *testing.B) {
	p := NewParams("n1", "tcp://1.2.3.4:2376", benchCA, benchCert, benchKey)
	for b.Loop() {
		benchSink = legacyCacheKey(p)
	}
}

func BenchmarkEndpointCacheKey(b *testing.B) {
	for b.Loop() {
		benchSink = EndpointCacheKey("tcp://1.2.3.4:2376", benchCA, benchCert, benchKey)
	}
}

func BenchmarkEndpointCacheKeyLegacy(b *testing.B) {
	for b.Loop() {
		benchSink = legacyCacheKey(NewParams("", "tcp://1.2.3.4:2376", benchCA, benchCert, benchKey))
	}
}

func legacyCacheKey(p *Params) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf(":%+v:%+v:%+v", p.CA, p.Cert, p.Key)))
	return fmt.Sprintf("%+v-%+v", p.Endpoint, hex.EncodeToString(sum[:])[:8])
}
