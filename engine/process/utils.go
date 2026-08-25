package process

import (
	"crypto/rand"
	"encoding/hex"
	"net"
	"net/url"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/cockroachdb/errors"

	coretypes "github.com/projecteru2/core/types"
)

const (
	unitPrefix  = "eru-"
	unitSuffix  = ".service"
	sliceSuffix = ".slice"
	cgroupRoot  = "/sys/fs/cgroup"
	imageCache  = "_images"
	idBytes     = 16
)

var (
	// systemd expands % specifiers in unit settings, so a literal one has to be doubled.
	envEscaper = strings.NewReplacer(`\`, `\\`, `"`, `\"`, "%", "%%")

	podnamePattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)
)

// parseEndpoint splits process://[user@]host[:port] into its ssh user, host and dial address.
func parseEndpoint(endpoint string) (user, host, addr string, err error) {
	target, ok := strings.CutPrefix(endpoint, Prefix)
	if !ok {
		return "", "", "", errors.Wrapf(coretypes.ErrInvaildEngineEndpoint, "endpoint %s", endpoint)
	}
	if name, rest, found := strings.Cut(target, "@"); found {
		user, target = name, rest
	}
	host, port, err := net.SplitHostPort(target)
	if err != nil {
		host, port = strings.Trim(target, "[]"), defaultPort
	}
	if host == "" || port == "" {
		return "", "", "", errors.Wrapf(coretypes.ErrInvaildEngineEndpoint, "endpoint %s", endpoint)
	}
	return user, host, net.JoinHostPort(host, port), nil
}

// quote renders argv as a shell line with every word single-quoted.
func quote(argv []string) string {
	words := make([]string, len(argv))
	for i, arg := range argv {
		words[i] = "'" + strings.ReplaceAll(arg, "'", `'\''`) + "'"
	}
	return strings.Join(words, " ")
}

// shell wraps a script body into an argv whose positional parameters carry args.
func shell(body string, args ...string) []string {
	return slices.Concat([]string{"sh", "-c", body, "sh"}, args)
}

func unitName(ID string) string {
	return unitPrefix + ID + unitSuffix
}

func sliceName(podname string) string {
	return unitPrefix + podname + sliceSuffix
}

func workloadDir(root, ID string) string {
	return filepath.Join(root, ID)
}

func imageDir(root, ref string) string {
	return filepath.Join(root, imageCache, url.PathEscape(ref))
}

func metaPath(ID string) string {
	return filepath.Join(metaDir, ID+".json")
}

// recordPath is the meta file's durable copy; the one under metaDir lives on tmpfs.
func recordPath(root, ID string) string {
	return filepath.Join(workloadDir(root, ID), "meta.json")
}

// cgroupPath expands a slice name into its cgroup directory; systemd nests slices on "-".
func cgroupPath(slice, unit string) string {
	parts := strings.Split(strings.TrimSuffix(slice, sliceSuffix), "-")
	segments := make([]string, 0, len(parts)+2)
	segments = append(segments, cgroupRoot)
	for i := range parts {
		segments = append(segments, strings.Join(parts[:i+1], "-")+sliceSuffix)
	}
	return filepath.Join(append(segments, unit)...)
}

func newID() string {
	buf := make([]byte, idBytes)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}

// restartPolicy maps eru's docker-shaped restart string onto systemd's Restart=.
func restartPolicy(restart string) string {
	policy, _, _ := strings.Cut(restart, ":")
	switch policy {
	case "always", "unless-stopped":
		return "always"
	case "on-failure":
		return "on-failure"
	default:
		return ""
	}
}

// systemdEnv quotes one KEY=VALUE pair, as systemd splits Environment= on whitespace.
func systemdEnv(entry string) string {
	key, value, ok := strings.Cut(entry, "=")
	if !ok {
		return entry
	}
	return key + `="` + envEscaper.Replace(value) + `"`
}

// splitRef separates an image reference from its tag, ignoring a registry port.
func splitRef(ref string) (name, tag string) {
	colon := strings.LastIndex(ref, ":")
	if colon < 0 || colon < strings.LastIndex(ref, "/") {
		return ref, ""
	}
	return ref[:colon], ref[colon+1:]
}

func lastEnvValue(env []string, key string) string {
	last := ""
	for _, entry := range env {
		if name, value, ok := strings.Cut(entry, "="); ok && name == key {
			last = value
		}
	}
	return last
}

func validPodname(podname string) bool {
	return podnamePattern.MatchString(podname)
}

func parseShow(out string) map[string]string {
	shown := map[string]string{}
	for line := range strings.Lines(out) {
		if key, value, ok := strings.Cut(strings.TrimRight(line, "\n"), "="); ok {
			shown[key] = value
		}
	}
	return shown
}

func exitError(argv []string, res *result) error {
	if res.Code == 0 {
		return nil
	}
	return errors.Newf("%s exited %d: %s", argv[0], res.Code, strings.TrimSpace(res.Stderr))
}
