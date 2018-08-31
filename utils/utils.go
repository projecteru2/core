package utils

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"strings"

	enginecontainer "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	engineapi "github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/projecteru2/core/cluster"
	"github.com/projecteru2/core/types"
	log "github.com/sirupsen/logrus"
)

type ctxKey string

const (
	letters              = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	shortenLength        = 7
	engineKey     ctxKey = "engine"
	// DefaultVersion for default version
	DefaultVersion = "latest"
	// WrongVersion for wrong version
	WrongVersion = "unknown"
)

// RandomString random a string
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

// TruncateID truncate container ID
func TruncateID(id string) string {
	if len(id) > shortenLength {
		return id[:shortenLength]
	}
	return id
}

// Tail return tail thing
func Tail(path string) string {
	parts := strings.Split(path, "/")
	return parts[len(parts)-1]
}

// FuckDockerStream will copy docker stream to stdout and err
func FuckDockerStream(stream io.ReadCloser) io.Reader {
	outr, outw := io.Pipe()

	go func() {
		defer stream.Close()
		_, err := stdcopy.StdCopy(outw, outw, stream)
		outw.CloseWithError(err)
	}()

	return outr
}

// GetGitRepoName return git repo name
func GetGitRepoName(url string) (string, error) {
	if !(strings.Contains(url, "git@") || strings.Contains(url, "gitlab@")) || !strings.HasSuffix(url, ".git") {
		return "", fmt.Errorf("Bad git url format %q, only ssh protocol support", url)
	}

	return strings.TrimSuffix(Tail(url), ".git"), nil
}

// GetTag reture image tag
func GetTag(image string) string {
	if !strings.Contains(image, ":") {
		return DefaultVersion
	}

	parts := strings.Split(image, ":")
	if len(parts) != 2 {
		return WrongVersion
	}

	return parts[len(parts)-1]
}

// NormalizeImageName will normalize image name
func NormalizeImageName(image string) string {
	if !strings.Contains(image, ":") {
		return fmt.Sprintf("%s:latest", image)
	}
	return image
}

// ContextWithDockerEngine bind docker engine to context
// Bind a docker engine client to context
func ContextWithDockerEngine(ctx context.Context, client *engineapi.Client) context.Context {
	return context.WithValue(ctx, engineKey, client)
}

// GetDockerEngineFromContext get docker engine from context
// Get a docker engine client from a context
func GetDockerEngineFromContext(ctx context.Context) (*engineapi.Client, bool) {
	client, ok := ctx.Value(engineKey).(*engineapi.Client)
	return client, ok
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

// MakeCommandLineArgs make command line args
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

// MakeContainerName joins appname, entrypoint, ident using '_'
func MakeContainerName(appname, entrypoint, ident string) string {
	return strings.Join([]string{appname, entrypoint, ident}, "_")
}

// ParseContainerName does the opposite thing as MakeContainerName
func ParseContainerName(containerName string) (string, string, string, error) {
	containerName = strings.TrimLeft(containerName, "/")
	splits := strings.Split(containerName, "_")
	length := len(splits)
	if length >= 3 {
		return strings.Join(splits[0:length-2], "_"), splits[length-2], splits[length-1], nil
	}
	return "", "", "", fmt.Errorf("Bad containerName: %s", containerName)
}

// MakePublishInfo generate publish info
func MakePublishInfo(networks map[string]*network.EndpointSettings, node *types.Node, ports []string) map[string][]string {
	result := map[string][]string{}
	for networkName, networkSetting := range networks {
		ip := networkSetting.IPAddress
		if enginecontainer.NetworkMode(networkName).IsHost() {
			ip = node.GetIP()
		}

		data := []string{}
		for _, port := range ports {
			data = append(data, fmt.Sprintf("%s:%s", ip, port))
		}
		if len(data) > 0 {
			result[networkName] = data
		}
	}
	return result
}

// EncodePublishInfo encode publish info
func EncodePublishInfo(info map[string][]string) map[string]string {
	result := map[string]string{}
	for nm, publishs := range info {
		if len(publishs) > 0 {
			result[nm] = strings.Join(publishs, ",")
		}
	}
	return result
}

// DecodePublishInfo decode publish info
func DecodePublishInfo(info map[string]string) map[string][]string {
	result := map[string][]string{}
	for nm, publishs := range info {
		if publishs != "" {
			result[nm] = strings.Split(publishs, ",")
		}
	}
	return result
}

// EncodeMetaInLabel encode meta to json
func EncodeMetaInLabel(meta *types.EruContainerMeta) string {
	data, err := json.Marshal(meta)
	if err != nil {
		log.Errorf("[EncodeMetaInLabel] Encode meta failed %v", err)
		return ""
	}
	return string(data)
}

// DecodeMetaInLabel get meta from label and decode it
func DecodeMetaInLabel(labels map[string]string) *types.EruContainerMeta {
	meta := &types.EruContainerMeta{}
	metastr, ok := labels[cluster.ERUMeta]
	if ok {
		if err := json.Unmarshal([]byte(metastr), meta); err != nil {
			log.Errorf("[DecodeMetaInLabel] Decode failed %v", err)
		}
	}
	return meta
}
