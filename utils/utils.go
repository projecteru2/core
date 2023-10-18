package utils

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/big"
	"os"
	"strings"
	"time"
	"unsafe"

	"github.com/cockroachdb/errors"
	"github.com/projecteru2/core/cluster"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
)

const (
	letters       = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	shortenLength = 7
	// DefaultVersion for default version
	DefaultVersion = "latest"
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

// Tail return tail thing
func Tail(path string) string {
	return path[strings.LastIndex(path, "/")+1:]
}

// GetGitRepoName return git repo name
func GetGitRepoName(url string) (string, error) {
	if !(strings.Contains(url, "git@") || strings.Contains(url, "gitlab@") || strings.Contains(url, "https://")) || !strings.HasSuffix(url, ".git") {
		return "", errors.Wrap(types.ErrInvalidGitURL, url)
	}

	return strings.TrimSuffix(Tail(url), ".git"), nil
}

// GetTag reture image tag
func GetTag(image string) string {
	if !strings.Contains(image, ":") {
		return DefaultVersion
	}
	return image[strings.LastIndex(image, ":")+1:]
}

// NormalizeImageName will normalize image name
func NormalizeImageName(image string) string {
	if !strings.Contains(image, ":") {
		return fmt.Sprintf("%s:latest", image)
	}
	return image
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

// MakeWorkloadName joins appname, entrypoint, ident using '_'
func MakeWorkloadName(appname, entrypoint, ident string) string {
	return strings.Join([]string{appname, entrypoint, ident}, "_")
}

// ParseWorkloadName does the opposite thing as MakeWorkloadName
func ParseWorkloadName(workloadName string) (string, string, string, error) {
	workloadName = strings.TrimLeft(workloadName, "/")
	splits := strings.Split(workloadName, "_")
	length := len(splits)
	if length >= 3 {
		return strings.Join(splits[0:length-2], "_"), splits[length-2], splits[length-1], nil
	}
	return "", "", "", errors.Wrap(types.ErrInvalidWorkloadName, workloadName)
}

// MakePublishInfo generate publish info
func MakePublishInfo(networks map[string]string, ports []string) map[string][]string {
	result := map[string][]string{}
	for networkName, ip := range networks {
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
func EncodeMetaInLabel(ctx context.Context, meta *types.LabelMeta) string {
	data, err := json.Marshal(meta)
	if err != nil {
		log.WithFunc("utils.EncodeMetaInLabel").Error(ctx, err, "Encode meta failed")
		return ""
	}
	return string(data)
}

// DecodeMetaInLabel get meta from label and decode it
func DecodeMetaInLabel(ctx context.Context, labels map[string]string) *types.LabelMeta {
	meta := &types.LabelMeta{}
	metastr, ok := labels[cluster.LabelMeta]
	if ok {
		if err := json.Unmarshal([]byte(metastr), meta); err != nil {
			log.WithFunc("utils.DecodeMetaInLabel").Error(ctx, err, "Decode failed")
		}
	}
	return meta
}

// ShortID short workload ID
func ShortID(workloadID string) string {
	return workloadID[Max(0, len(workloadID)-shortenLength):]
}

// LabelsFilter filter workload by labels
func LabelsFilter(extend map[string]string, labels map[string]string) bool {
	for k, v := range labels {
		if n, ok := extend[k]; !ok || n != v {
			return false
		}
	}
	return true
}

// CleanStatsdMetrics trans dot to _
func CleanStatsdMetrics(k string) string {
	return strings.ReplaceAll(k, ".", "-")
}

// TempFile store a temp file
func TempFile(stream io.ReadCloser) (string, error) {
	f, err := os.CreateTemp(os.TempDir(), "")
	if err != nil {
		return "", err
	}
	defer f.Close()
	defer stream.Close()

	_, err = io.Copy(f, stream)
	return f.Name(), err
}

// Round for float64 to int
func Round(f float64) float64 {
	return math.Round(f*1000000000) / 1000000000
}

// MergeHookOutputs merge hooks output
func MergeHookOutputs(outputs []*bytes.Buffer) []byte {
	r := []byte{}
	for _, m := range outputs {
		r = append(r, m.Bytes()...)
	}
	return r
}

// EnsureReaderClosed As the name says,
// blocks until the stream is empty, until we meet EOF
func EnsureReaderClosed(ctx context.Context, stream io.ReadCloser) {
	if stream == nil {
		return
	}
	if _, err := io.Copy(io.Discard, stream); err != nil {
		log.WithFunc("utils.EnsureReaderClosed").Error(ctx, err, "Empty stream failed")
	}
	_ = stream.Close()
}

// Range .
func Range(n int) (res []int) {
	for i := 0; i < n; i++ {
		res = append(res, i)
	}
	return
}

// WithTimeout runs a function with given timeout
func WithTimeout(ctx context.Context, timeout time.Duration, f func(context.Context)) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	f(ctx)
}

// SHA256 .
func SHA256(input string) string {
	c := sha256.New()
	c.Write([]byte(input))
	bytes := c.Sum(nil)
	return hex.EncodeToString(bytes)
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
			continue
		}
		if !strings.HasSuffix(i, inquote) {
			block += i + " "
		} else {
			block += strings.TrimSuffix(i, inquote)
			inquote = ""
			result = append(result, block)
			block = ""
		}
	}

	return result
}

func Bool2Int(a bool) int {
	return *(*int)(unsafe.Pointer(&a)) & 1
}
