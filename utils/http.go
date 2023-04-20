package utils

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/alphadose/haxmap"
	"github.com/docker/go-connections/tlsconfig"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
)

var defaultHTTPClient = &http.Client{
	CheckRedirect: checkRedirect,
	Transport:     getDefaultTransport(),
}

var defaultUnixSockClient = &http.Client{
	Transport: getDefaultUnixSockTransport(),
}

var httpsClientCache = haxmap.New[string, *http.Client]()

// GetHTTPClient returns a HTTP client
func GetHTTPClient() *http.Client {
	return defaultHTTPClient
}

// GetUnixSockClient .
func GetUnixSockClient() *http.Client {
	return defaultUnixSockClient
}

// GetHTTPSClient returns an HTTPS client
// if cert_path/ca/cert/key is empty, it returns an HTTP client instead
func GetHTTPSClient(ctx context.Context, certPath, name, ca, cert, key string) (client *http.Client, err error) {
	if certPath == "" || ca == "" || cert == "" || key == "" {
		return GetHTTPClient(), nil
	}

	cacheKey := name + SHA256(fmt.Sprintf("%s-%s-%s-%s-%s", certPath, name, ca, cert, key))[:8]
	if httpsClient, ok := httpsClientCache.Get(cacheKey); ok {
		return httpsClient, nil
	}

	caFile, err := os.CreateTemp(certPath, fmt.Sprintf("ca-%s", name))
	if err != nil {
		return nil, err
	}
	certFile, err := os.CreateTemp(certPath, fmt.Sprintf("cert-%s", name))
	if err != nil {
		return nil, err
	}
	keyFile, err := os.CreateTemp(certPath, fmt.Sprintf("key-%s", name))
	if err != nil {
		return nil, err
	}
	if err = dumpFromString(ctx, caFile, certFile, keyFile, ca, cert, key); err != nil {
		return nil, err
	}
	options := tlsconfig.Options{
		CAFile:             caFile.Name(),
		CertFile:           certFile.Name(),
		KeyFile:            keyFile.Name(),
		InsecureSkipVerify: true,
	}
	defer os.Remove(caFile.Name())
	defer os.Remove(certFile.Name())
	defer os.Remove(keyFile.Name())
	tlsc, err := tlsconfig.Client(options)
	if err != nil {
		return nil, err
	}
	transport := getDefaultTransport()
	transport.TLSClientConfig = tlsc

	client = &http.Client{
		CheckRedirect: checkRedirect,
		Transport:     transport,
	}
	httpsClientCache.Set(cacheKey, client)
	return client, nil
}

func getDefaultTransport() *http.Transport {
	return &http.Transport{
		DialContext: (&net.Dialer{
			KeepAlive: time.Second * 30,
			Timeout:   time.Second * 30,
		}).DialContext,

		IdleConnTimeout:     time.Second * 90,
		MaxIdleConnsPerHost: runtime.GOMAXPROCS(0) + 1,
		Proxy:               http.ProxyFromEnvironment,
	}
}

func getDefaultUnixSockTransport() *http.Transport {
	return &http.Transport{
		DialContext: func(_ context.Context, _, addr string) (net.Conn, error) {
			return net.DialTimeout("unix", strings.Split(addr, ":")[0], time.Second*30)
		},

		IdleConnTimeout:     time.Second * 90,
		MaxIdleConnsPerHost: runtime.GOMAXPROCS(0) + 1,
		DisableCompression:  true,
	}
}

func dumpFromString(ctx context.Context, ca, cert, key *os.File, caStr, certStr, keyStr string) error {
	files := []*os.File{ca, cert, key}
	data := []string{caStr, certStr, keyStr}
	for i := 0; i < 3; i++ {
		if _, err := files[i].WriteString(data[i]); err != nil {
			return err
		}
		if err := files[i].Chmod(0444); err != nil {
			return err
		}
		if err := files[i].Close(); err != nil {
			return err
		}
	}
	log.WithFunc("utils.dumpFromString").Debug(ctx, "Dump ca.pem, cert.pem, key.pem from string")
	return nil
}

func checkRedirect(_ *http.Request, via []*http.Request) error {
	if via[0].Method == http.MethodGet {
		return http.ErrUseLastResponse
	}
	return types.ErrUnexpectedRedirect
}
