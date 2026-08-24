package types

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

type Params struct {
	Nodename string
	Endpoint string
	CA       string
	Cert     string
	Key      string
}

func (p *Params) CacheKey() string {
	return fmt.Sprintf("%+v-%+v", p.Endpoint, sha256String(fmt.Sprintf(":%+v:%+v:%+v", p.CA, p.Cert, p.Key))[:8])
}

// utils.SHA256 would create an import cycle
func sha256String(input string) string {
	c := sha256.New()
	c.Write([]byte(input))
	bytes := c.Sum(nil)
	return hex.EncodeToString(bytes)
}
