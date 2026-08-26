package utils

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"strings"
	"time"

	"github.com/cockroachdb/errors"

	"github.com/projecteru2/core/cluster"
	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
)

const (
	letters       = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	shortenLength = 7
	idBytes       = 16
)

// RandomString returns n random letters from [a-zA-Z].
func RandomString(n int) string {
	buf := make([]byte, n)
	_, _ = rand.Read(buf)
	for i, b := range buf {
		buf[i] = letters[int(b)%len(letters)]
	}
	return string(buf)
}

func RandomID() string {
	buf := make([]byte, idBytes)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}

// Tail returns the segment of path after the last "/".
func Tail(path string) string {
	return path[strings.LastIndex(path, "/")+1:]
}

func GetGitRepoName(url string) (string, error) {
	if (!strings.Contains(url, "git@") && !strings.Contains(url, "gitlab@") && !strings.Contains(url, "https://")) || !strings.HasSuffix(url, ".git") {
		return "", errors.Wrap(types.ErrInvalidGitURL, url)
	}

	return strings.TrimSuffix(Tail(url), ".git"), nil
}

// MakeCommandLineArgs splits s into argv, honoring single and double quotes.
func MakeCommandLineArgs(s string) []string {
	result := []string{}
	var inquote, block string
	for part := range strings.SplitSeq(s, " ") {
		if inquote == "" {
			switch {
			case strings.HasPrefix(part, "'") || strings.HasPrefix(part, "\""):
				inquote = string(part[0])
				block = strings.TrimPrefix(part, inquote) + " "
			case part != "":
				result = append(result, part)
			}
			continue
		}
		if !strings.HasSuffix(part, inquote) {
			block += part + " "
			continue
		}
		block += strings.TrimSuffix(part, inquote)
		inquote = ""
		if block != "" {
			result = append(result, block)
		}
		block = ""
	}
	return result
}

// MakeWorkloadName joins appname, entrypoint, ident using '_'
func MakeWorkloadName(appname, entrypoint, ident string) string {
	return strings.Join([]string{appname, entrypoint, ident}, "_")
}

// ParseWorkloadName is the inverse of MakeWorkloadName.
func ParseWorkloadName(workloadName string) (string, string, string, error) {
	workloadName = strings.TrimLeft(workloadName, "/")
	splits := strings.Split(workloadName, "_")
	length := len(splits)
	if length >= 3 {
		return strings.Join(splits[0:length-2], "_"), splits[length-2], splits[length-1], nil
	}
	return "", "", "", errors.Wrap(types.ErrInvalidWorkloadName, workloadName)
}

// MakePublishInfo maps each network to its "ip:port" strings.
func MakePublishInfo(networks map[string]string, ports []string) map[string][]string {
	result := map[string][]string{}
	for networkName, ip := range networks {
		data := make([]string, 0, len(ports))
		for _, port := range ports {
			data = append(data, fmt.Sprintf("%s:%s", ip, port))
		}
		if len(data) > 0 {
			result[networkName] = data
		}
	}
	return result
}

// EncodePublishInfo joins each network's addresses with commas.
func EncodePublishInfo(info map[string][]string) map[string]string {
	result := map[string]string{}
	for nm, publishs := range info {
		if len(publishs) > 0 {
			result[nm] = strings.Join(publishs, ",")
		}
	}
	return result
}

// EncodeMetaInLabel returns meta as JSON, or "" when marshaling fails.
func EncodeMetaInLabel(ctx context.Context, meta *types.LabelMeta) string {
	data, err := json.Marshal(meta)
	if err != nil {
		log.WithFunc("utils.EncodeMetaInLabel").Error(ctx, err, "encode meta")
		return ""
	}
	return string(data)
}

// DecodeMetaInLabel returns the meta from labels, or an empty meta when absent or malformed.
func DecodeMetaInLabel(ctx context.Context, labels map[string]string) *types.LabelMeta {
	meta := &types.LabelMeta{}
	metastr, ok := labels[cluster.LabelMeta]
	if ok {
		if err := json.Unmarshal([]byte(metastr), meta); err != nil {
			log.WithFunc("utils.DecodeMetaInLabel").Error(ctx, err, "decode meta in label")
		}
	}
	return meta
}

// NewHealthCheck renders the label's health check the way a workload record carries it.
func NewHealthCheck(check *types.HealthCheck) *enginetypes.HealthCheck {
	if check == nil {
		return nil
	}
	return &enginetypes.HealthCheck{
		TCPPorts: check.TCPPorts,
		HTTPPort: check.HTTPPort,
		HTTPURL:  check.HTTPURL,
		HTTPCode: check.HTTPCode,
	}
}

// ShortID returns the last 7 characters of workloadID.
func ShortID(workloadID string) string {
	return workloadID[max(0, len(workloadID)-shortenLength):]
}

// LabelsFilter reports whether every key/value in labels is present in extend.
func LabelsFilter(extend, labels map[string]string) bool {
	for k, v := range labels {
		if n, ok := extend[k]; !ok || n != v {
			return false
		}
	}
	return true
}

// CleanStatsdMetrics replaces "." with "-".
func CleanStatsdMetrics(k string) string {
	return strings.ReplaceAll(k, ".", "-")
}

// Round rounds f to 9 decimal places.
func Round(f float64) float64 {
	return math.Round(f*1000000000) / 1000000000
}

func MergeHookOutputs(outputs []*bytes.Buffer) []byte {
	r := []byte{}
	for _, m := range outputs {
		r = append(r, m.Bytes()...)
	}
	return r
}

// EnsureReaderClosed drains stream to EOF and closes it; a nil stream is a no-op.
func EnsureReaderClosed(ctx context.Context, stream io.ReadCloser) {
	if stream == nil {
		return
	}
	if _, err := io.Copy(io.Discard, stream); err != nil {
		log.WithFunc("utils.EnsureReaderClosed").Error(ctx, err, "drain stream")
	}
	_ = stream.Close()
}

// Range returns []int{0, 1, ..., n-1}.
func Range(n int) []int {
	res := make([]int, n)
	for i := range n {
		res[i] = i
	}
	return res
}

// WithTimeout runs a function with given timeout
func WithTimeout(ctx context.Context, timeout time.Duration, f func(context.Context)) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	f(ctx)
}

func Bool2Int(a bool) int {
	if a {
		return 1
	}
	return 0
}

// VolumeBind is one host bind parsed from "source:dest[:ro]" after env expansion.
type VolumeBind struct {
	Source   string
	Dest     string
	ReadOnly bool
}

// ParseVolumeBinds keeps the well-formed binds and expands $VAR from env.
func ParseVolumeBinds(volumes, env []string) []VolumeBind {
	lookup := make(map[string]string, len(env))
	for _, entry := range env {
		if key, value, ok := strings.Cut(entry, "="); ok {
			lookup[key] = value
		}
	}
	expand := func(key string) string { return lookup[key] }
	binds := make([]VolumeBind, 0, len(volumes))
	for _, volume := range volumes {
		parts := strings.Split(os.Expand(volume, expand), ":")
		if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
			continue
		}
		binds = append(binds, VolumeBind{Source: parts[0], Dest: parts[1], ReadOnly: len(parts) > 2 && parts[2] == "ro"})
	}
	return binds
}

// ParseRate reads a human byte rate, treating junk and negatives as zero.
func ParseRate(rate string) int64 {
	parsed, err := ParseRAMInHuman(rate)
	if err != nil || parsed < 0 {
		return 0
	}
	return parsed
}

// LastEnvValue takes the last value of key, as core appends its own after the client's.
func LastEnvValue(env []string, key string) string {
	last := ""
	for _, entry := range env {
		if name, value, ok := strings.Cut(entry, "="); ok && name == key {
			last = value
		}
	}
	return last
}
