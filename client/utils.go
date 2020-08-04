package client

import (
	"fmt"
	"strings"
)

func makeTarget(addr string) string {
	scheme := "eru"
	if strings.HasPrefix(addr, "static://") {
		scheme = "static"
		addr = strings.TrimPrefix(addr, "static://")
	}
	return fmt.Sprintf("%s://_/%s", scheme, addr)
}
