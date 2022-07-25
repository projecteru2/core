package utils

import (
	"fmt"
	"net"
	"strings"
)

// GetOutboundAddress finds out self-service address
func GetOutboundAddress(bind string, dialTarget string) (string, error) {
	parts := strings.Split(bind, ":")
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid bind address %s", bind)
	}
	ip := parts[0]
	port := parts[1]

	address := net.ParseIP(ip)
	if ip == "" || address == nil || address.IsUnspecified() {
		return getOutboundAddress(port, dialTarget)
	}

	return bind, nil
}

func getOutboundAddress(port string, dialTarget string) (string, error) {
	conn, err := net.Dial("udp", dialTarget)
	if err != nil {
		return "", err
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return fmt.Sprintf("%s:%s", localAddr.IP, port), nil
}
