package utils

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"math/big"
	"os"
	"strings"

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
		return "", types.NewDetailedErr(types.ErrInvalidGitURL, url)
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
	return "", "", "", types.NewDetailedErr(types.ErrInvalidWorkloadName, workloadName)
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
func EncodeMetaInLabel(meta *types.LabelMeta) string {
	data, err := json.Marshal(meta)
	if err != nil {
		log.Errorf("[EncodeMetaInLabel] Encode meta failed %v", err)
		return ""
	}
	return string(data)
}

// DecodeMetaInLabel get meta from label and decode it
func DecodeMetaInLabel(labels map[string]string) *types.LabelMeta {
	meta := &types.LabelMeta{}
	metastr, ok := labels[cluster.LabelMeta]
	if ok {
		if err := json.Unmarshal([]byte(metastr), meta); err != nil {
			log.Errorf("[DecodeMetaInLabel] Decode failed %v", err)
		}
	}
	return meta
}

// ShortID short workload ID
func ShortID(workloadID string) string {
	return workloadID[:Min(len(workloadID), shortenLength)]
}

// FilterWorkload filter workload by labels
func FilterWorkload(extend map[string]string, labels map[string]string) bool {
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
	f, err := ioutil.TempFile(os.TempDir(), "")
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
	return types.Round(f)
}

// MergeHookOutputs merge hooks output
func MergeHookOutputs(outputs []*bytes.Buffer) []byte {
	r := []byte{}
	for _, m := range outputs {
		r = append(r, m.Bytes()...)
	}
	return r
}

// Min returns the lesser one.
func Min(x int, xs ...int) int {
	if len(xs) == 0 {
		return x
	}
	if m := Min(xs[0], xs[1:]...); m < x {
		return m
	}
	return x
}

// Min64 return lesser one
func Min64(x int64, xs ...int64) int64 {
	if len(xs) == 0 {
		return x
	}
	if m := Min64(xs[0], xs[1:]...); m < x {
		return m
	}
	return x
}

// Max returns the biggest int
func Max(x int, xs ...int) int {
	if len(xs) == 0 {
		return x
	}
	if m := Max(xs[0], xs[1:]...); m > x {
		return m
	}
	return x
}

// EnsureReaderClosed As the name says,
// blocks until the stream is empty, until we meet EOF
func EnsureReaderClosed(stream io.ReadCloser) {
	if stream == nil {
		return
	}
	if _, err := io.Copy(ioutil.Discard, stream); err != nil {
		log.Errorf("[EnsureReaderClosed] empty stream failed %v", err)
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
