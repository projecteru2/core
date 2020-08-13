package utils

import (
	"fmt"
	"strings"
)

var legitSchemes map[string]string = map[string]string{
	"eru":    "eru://",
	"static": "static://",
}

func MakeTarget(addr string) string {
	scheme := "eru"
	for s, prefix := range legitSchemes {
		if strings.HasPrefix(addr, prefix) {
			scheme = s
			addr = strings.TrimPrefix(addr, prefix)
			break
		}
	}

	return fmt.Sprintf("%s://_/%s", scheme, addr)
}
