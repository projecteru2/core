package utils

import (
	"fmt"
	"net"
	"strings"
)

func GetOutboundAddress(bind string) (string, error) {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "", err
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	port := strings.Split(bind, ":")[1]
	return fmt.Sprintf("%s:%s", localAddr.IP, port), nil
}
