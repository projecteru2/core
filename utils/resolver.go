package utils

import (
	"fmt"
	"strings"

	"github.com/projecteru2/core/types"
)

var legitSchemas = []string{
	"eru://",
	"static://",
}

// MakeTarget return target: {scheme}://@{user}:{password}/{endpoint}
func MakeTarget(addr string, authConfig types.AuthConfig) string {
	schema := legitSchemas[0]
	for _, prefix := range legitSchemas {
		if strings.HasPrefix(addr, prefix) {
			schema = prefix
			addr = strings.TrimPrefix(addr, prefix)
			break
		}
	}

	return fmt.Sprintf("%s@%s:%s/%s", schema, authConfig.Username, authConfig.Password, addr)
}
