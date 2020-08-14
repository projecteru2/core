package utils

import (
	"fmt"
	"strings"

	"github.com/projecteru2/core/types"
)

var legitSchemes = map[string]string{
	"eru":    "eru://",
	"static": "static://",
}

// MakeTarget return target: {scheme}://@{user}:{password}/{endpoint}
func MakeTarget(addr string, authConfig types.AuthConfig) string {
	scheme := "eru"
	for s, prefix := range legitSchemes {
		if strings.HasPrefix(addr, prefix) {
			scheme = s
			addr = strings.TrimPrefix(addr, prefix)
			break
		}
	}

	return fmt.Sprintf("%s://@%s:%s/%s", scheme, authConfig.Username, authConfig.Password, addr)
}
