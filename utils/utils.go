package utils

import (
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	engineapi "github.com/docker/engine-api/client"
	"github.com/docker/go-connections/tlsconfig"
	"gitlab.ricebook.net/platform/core/types"
)

const (
	letters       = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	shortenLength = 7
)

func RandomString(n int) string {
	rand.Seed(time.Now().UnixNano())
	r := make([]byte, n)
	for i := 0; i < n; i++ {
		r[i] = letters[rand.Intn(len(letters))]
	}
	return string(r)
}

func TruncateID(id string) string {
	if len(id) > shortenLength {
		return id[:shortenLength]
	}
	return id
}

func Tail(path string) string {
	parts := strings.Split(path, "/")
	return parts[len(parts)-1]
}

func MakeDockerClient(endpoint, config *types.Config) (*engineapi.Client, error) {
	if !strings.HasPrefix(endpoint, "tcp://") {
		endpoint = "tcp://" + endpoint
	}

	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}

	host, _, err := net.SplitHostPort(u.Host)
	if err != nil {
		return nil, err
	}

	dockerCertPath := filepath.Join(config.DockerConfig.DockerCertPath, host)
	options := tlsconfig.Options{
		CAFile:             filepath.Join(dockerCertPath, "ca.pem"),
		CertFile:           filepath.Join(dockerCertPath, "cert.pem"),
		KeyFile:            filepath.Join(dockerCertPath, "key.pem"),
		InsecureSkipVerify: false,
	}
	tlsc, err := tlsconfig.Client(options)
	if err != nil {
		return nil, err
	}

	cli := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tlsc,
		},
	}

	return engineapi.NewClient(endpoint, config.DockerConfig.DockerAPIVersion, cli, nil)
}

func GetGitRepoName(url string) (string, error) {
	if !strings.HasPrefix("git@") || !strings.HasSuffix(".git") {
		return "", fmt.Errorf("Bad git url format %q", url)
	}

	x := strings.SplitN(url, ":", 2)
	if len(x) != 2 {
		return "", fmt.Errorf("Bad git url format %q", url)
	}

	y := strings.SplitN(x[1], "/", 2)
	if len(y) != 2 {
		return "", fmt.Errorf("Bad git url format %q", url)
	}
	return y[1][:len(y[1])-4]
}
