package types

import (
	"crypto/sha256"
	"encoding/hex"
)

type Params struct {
	Nodename string
	Endpoint string
	CA       string
	Cert     string
	Key      string

	cacheKey string
}

func NewParams(nodename, endpoint, ca, cert, key string) *Params {
	return &Params{
		Nodename: nodename,
		Endpoint: endpoint,
		CA:       ca,
		Cert:     cert,
		Key:      key,
		cacheKey: EndpointCacheKey(endpoint, ca, cert, key),
	}
}

func (p *Params) CacheKey() string {
	if p.cacheKey == "" {
		p.cacheKey = EndpointCacheKey(p.Endpoint, p.CA, p.Cert, p.Key)
	}
	return p.cacheKey
}

// EndpointCacheKey is the engine cache key for an endpoint and its credentials.
func EndpointCacheKey(endpoint, ca, cert, key string) string {
	// utils.SHA256 would create an import cycle
	sum := sha256.New()
	sum.Write([]byte{':'})
	sum.Write([]byte(ca))
	sum.Write([]byte{':'})
	sum.Write([]byte(cert))
	sum.Write([]byte{':'})
	sum.Write([]byte(key))
	return endpoint + "-" + hex.EncodeToString(sum.Sum(nil))[:8]
}
