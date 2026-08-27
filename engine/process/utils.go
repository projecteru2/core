package process

import (
	"net/url"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/projecteru2/core/engine/workloadmeta"
)

const (
	unitPrefix  = "eru-"
	unitSuffix  = ".service"
	sliceSuffix = ".slice"
	imageCache  = "_images"
	propsFile   = "props"
)

var (
	// systemd expands % specifiers in unit settings, so a literal one has to be doubled.
	envEscaper = strings.NewReplacer(`\`, `\\`, `"`, `\"`, "%", "%%")

	podnamePattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)
)

func unitName(ID string) string {
	return unitPrefix + ID + unitSuffix
}

func sliceName(podname string) string {
	return unitPrefix + podname + sliceSuffix
}

func workloadDir(root, ID string) string {
	return filepath.Join(root, ID)
}

func propsPath(dir string) string {
	return filepath.Join(dir, propsFile)
}

func imageDir(root, ref string) string {
	return filepath.Join(root, imageCache, url.PathEscape(ref))
}

// cgroupPath expands a slice name into its cgroup directory; systemd nests slices on "-".
func cgroupPath(slice, unit string) string {
	parts := strings.Split(strings.TrimSuffix(slice, sliceSuffix), "-")
	segments := make([]string, 0, len(parts)+2)
	segments = append(segments, workloadmeta.CgroupRoot)
	for i := range parts {
		segments = append(segments, strings.Join(parts[:i+1], "-")+sliceSuffix)
	}
	return filepath.Join(append(segments, unit)...)
}

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
