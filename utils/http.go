package utils

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/docker/go-connections/tlsconfig"
	"golang.org/x/sync/singleflight"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
)

var (
	defaultHTTPClient = &http.Client{
		CheckRedirect: checkRedirect,
		Transport:     getDefaultTransport(),
	}

	defaultUnixSockClient = &http.Client{
		Transport: getDefaultUnixSockTransport(),
	}

	httpsClientCache sync.Map
	httpsClientGroup singleflight.Group
)

func GetHTTPClient() *http.Client {
	return defaultHTTPClient
}

func GetUnixSockClient() *http.Client {
	return defaultUnixSockClient
}

// GetHTTPSClient returns a per-cert cached HTTPS client, or the plain HTTP client when any of certPath/ca/cert/key is empty.
func GetHTTPSClient(ctx context.Context, certPath, name, ca, cert, key string) (*http.Client, error) {
	if certPath == "" || ca == "" || cert == "" || key == "" {
		return GetHTTPClient(), nil
	}

	cacheKey := name + SHA256(fmt.Sprintf("%s-%s-%s-%s-%s", certPath, name, ca, cert, key))[:8]
	if client, ok := httpsClientCache.Load(cacheKey); ok {
		return client.(*http.Client), nil
	}

	built, err, _ := httpsClientGroup.Do(cacheKey, func() (any, error) {
		if client, ok := httpsClientCache.Load(cacheKey); ok {
			return client, nil
		}
		client, err := newHTTPSClient(ctx, certPath, name, ca, cert, key)
		if err != nil {
			return nil, err
		}
		httpsClientCache.Store(cacheKey, client)
		return client, nil
	})
	if err != nil {
		return nil, err
	}
	return built.(*http.Client), nil
}

func newHTTPSClient(ctx context.Context, certPath, name, ca, cert, key string) (*http.Client, error) {
	caFile, err := os.CreateTemp(certPath, fmt.Sprintf("ca-%s", name))
	if err != nil {
		return nil, err
	}
	defer discard(caFile)
	certFile, err := os.CreateTemp(certPath, fmt.Sprintf("cert-%s", name))
	if err != nil {
		return nil, err
	}
	defer discard(certFile)
	keyFile, err := os.CreateTemp(certPath, fmt.Sprintf("key-%s", name))
	if err != nil {
		return nil, err
	}
	defer discard(keyFile)
	if err = dumpFromString(ctx, caFile, certFile, keyFile, ca, cert, key); err != nil {
		return nil, err
	}
	tlsc, err := tlsconfig.Client(tlsconfig.Options{
		CAFile:             caFile.Name(),
		CertFile:           certFile.Name(),
		KeyFile:            keyFile.Name(),
		InsecureSkipVerify: true,
	})
	if err != nil {
		return nil, err
	}
	transport := getDefaultTransport()
	transport.TLSClientConfig = tlsc

	return &http.Client{
		CheckRedirect: checkRedirect,
		Transport:     transport,
	}, nil
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
	for i := range files {
		if _, err := files[i].WriteString(data[i]); err != nil {
			return err
		}
		if err := files[i].Chmod(0o444); err != nil {
			return err
		}
		if err := files[i].Close(); err != nil {
			return err
		}
	}
	log.WithFunc("utils.dumpFromString").Debug(ctx, "dump ca/cert/key from string")
	return nil
}

func checkRedirect(_ *http.Request, via []*http.Request) error {
	if via[0].Method == http.MethodGet {
		return http.ErrUseLastResponse
	}
	return types.ErrUnexpectedRedirect
}

func discard(f *os.File) {
	_ = f.Close()
	_ = os.Remove(f.Name())
}
