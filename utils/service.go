package utils

import (
	"fmt"
	"net"

	"github.com/cockroachdb/errors"

	"github.com/projecteru2/core/types"
)

// GetOutboundAddress returns bind, or the local IP reaching probeTarget when bind's host is unspecified.
func GetOutboundAddress(bind, probeTarget string) (string, error) {
	ip, port, err := net.SplitHostPort(bind)
	if err != nil {
		return "", errors.Wrap(types.ErrInvaildIPWithPort, bind)
	}

	address := net.ParseIP(ip)
	if ip == "" || address == nil || address.IsUnspecified() {
		return getOutboundAddress(port, probeTarget)
	}

	return bind, nil
}

func getOutboundAddress(port, probeTarget string) (string, error) {
	conn, err := net.Dial("udp", probeTarget)
	if err != nil {
		return "", err
	}
	defer func() {
		_ = conn.Close()
	}()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return fmt.Sprintf("%s:%s", localAddr.IP, port), nil
}
