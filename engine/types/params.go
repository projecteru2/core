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
}

func NewParams(nodename, endpoint, ca, cert, key string) *Params {
	return &Params{
		Nodename: nodename,
		Endpoint: endpoint,
		CA:       ca,
		Cert:     cert,
		Key:      key,
	}
}

func (p *Params) CacheKey() string {
	return p.Endpoint + "-" + sha256String(":" + p.CA + ":" + p.Cert + ":" + p.Key)[:8]
}

// utils.SHA256 would create an import cycle
func sha256String(input string) string {
	sum := sha256.Sum256([]byte(input))
	return hex.EncodeToString(sum[:])
}
