package utils

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetHTTPClient(t *testing.T) {
	assert.NotNil(t, GetHTTPClient())
}

func TestGetUnixSockClient(t *testing.T) {
	assert.NotNil(t, GetUnixSockClient())
}

func TestGetHTTPSClient(t *testing.T) {
	ctx := t.Context()
	client, err := GetHTTPSClient(ctx, "", "abc", "", "", "")
	assert.NoError(t, err)
	assert.NotNil(t, client)

	client, err = GetHTTPSClient(ctx, os.TempDir(), "abc", "1", "2", "3")
	assert.Error(t, err)
	assert.Nil(t, client)
}

func TestGetHTTPSClientBuildsOneClientPerCacheKey(t *testing.T) {
	ca, cert, key := selfSignedPEM(t)
	dir := t.TempDir()

	const callers = 8
	clients := make(chan *http.Client, callers)
	start := make(chan struct{})
	wg := &sync.WaitGroup{}
	for range callers {
		wg.Go(func() {
			<-start
			client, err := GetHTTPSClient(t.Context(), dir, "concurrent", ca, cert, key)
			if !assert.NoError(t, err) {
				return
			}
			clients <- client
		})
	}
	close(start)
	wg.Wait()
	close(clients)

	seen := map[*http.Client]struct{}{}
	for client := range clients {
		seen[client] = struct{}{}
	}
	assert.Len(t, seen, 1)
}

func TestCheckRedirect(t *testing.T) {
	via := []*http.Request{{Method: http.MethodGet}}
	err := checkRedirect(nil, via)
	assert.Equal(t, err, http.ErrUseLastResponse)
}

func selfSignedPEM(t *testing.T) (ca, cert, key string) {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "eru-core-test"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &priv.PublicKey, priv)
	require.NoError(t, err)
	keyDER, err := x509.MarshalECPrivateKey(priv)
	require.NoError(t, err)

	certPEM := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
	return certPEM, certPEM, string(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}))
}
