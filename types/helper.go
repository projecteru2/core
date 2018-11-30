package types

import (
	"math"
	"net"
	"net/url"
)

// Round for float64 to int
func Round(f float64) float64 {
	return math.Round(f*100) / 100
}

func getEndpointHost(endpoint string) (string, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return "", err
	}

	host, _, err := net.SplitHostPort(u.Host)
	if err != nil {
		return "", err
	}
	return host, nil
}
