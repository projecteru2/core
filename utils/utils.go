package utils

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"os"
	"strings"

	log "github.com/Sirupsen/logrus"
	engineapi "github.com/docker/engine-api/client"
	"gitlab.ricebook.net/platform/core/g"
	"gitlab.ricebook.net/platform/core/types"
	"golang.org/x/net/context"
)

const (
	letters       = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	shortenLength = 7
	CpuPeriodBase = 100000
)

func RandomString(n int) string {
	r := make([]byte, n)
	for i := 0; i < n; i++ {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(letters))))
		// 没那么惨吧
		if err != nil {
			continue
		}
		r[i] = letters[n.Int64()]
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

func GetGitRepoName(url string) (string, error) {
	if !strings.HasPrefix(url, "git@") || !strings.HasSuffix(url, ".git") {
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
	return strings.TrimSuffix(y[1], ".git"), nil
}

func GetVersion(image string) string {
	if !strings.Contains(image, ":") {
		return "unknown"
	}

	parts := strings.Split(image, ":")
	if len(parts) != 2 {
		return "unknown"
	}

	return parts[1]
}

// Bind a docker engine client to context
func ToDockerContext(client *engineapi.Client) context.Context {
	return context.WithValue(context.Background(), "engine", client)
}

// Get a docker engine client from a context
func FromDockerContext(ctx context.Context) (*engineapi.Client, bool) {
	client, ok := ctx.Value("engine").(*engineapi.Client)
	return client, ok
}

func SaveFile(content, path string, mode os.FileMode) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE, mode)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.WriteString(content)
	return err
}

// copied from https://gist.github.com/jmervine/d88c75329f98e09f5c87
func safeSplit(s string) []string {
	split := strings.Split(s, " ")

	var result []string
	var inquote string
	var block string
	for _, i := range split {
		if inquote == "" {
			if strings.HasPrefix(i, "'") || strings.HasPrefix(i, "\"") {
				inquote = string(i[0])
				block = strings.TrimPrefix(i, inquote) + " "
			} else {
				result = append(result, i)
			}
		} else {
			if !strings.HasSuffix(i, inquote) {
				block += i + " "
			} else {
				block += strings.TrimSuffix(i, inquote)
				inquote = ""
				result = append(result, block)
				block = ""
			}
		}
	}

	return result
}

func MakeCommandLineArgs(s string) []string {
	r := []string{}
	for _, part := range safeSplit(s) {
		if len(part) == 0 {
			continue
		}
		r = append(r, part)
	}
	return r
}

func SendMemCap(cpumemmap map[string]types.CPUAndMem, tag string) {
	data := map[string]float64{}
	for node, cpuandmem := range cpumemmap {
		data[node] = float64(cpuandmem.MemCap)
	}
	clean_host := strings.Replace(g.Hostname, ".", "-", -1)

	err := g.Statsd.Send(data, clean_host, tag)
	if err != nil {
		log.Errorf("Error occured while sending data to statsd: %v", err)
	}
	return
}
