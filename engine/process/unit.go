package process

import (
	"fmt"
	"maps"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/projecteru2/core/engine"
	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/utils"
)

const (
	minMemory        = 4 * 1024 * 1024
	defaultCPUWeight = 100
	maxCPUWeight     = 10000
	quotaPercent     = 100
	readOnlyMode     = "ro"
	syslogIdentifier = "eru"
)

var throttleKeys = [...]string{"IOReadIOPSMax", "IOWriteIOPSMax", "IOReadBandwidthMax", "IOWriteBandwidthMax"}

// unit is the transient service that runs one workload.
type unit struct {
	ID          string
	Podname     string
	User        string
	Root        string // RootDirectory, empty for a raw host process
	Bundle      string // the unpacked artifact, where a raw process resolves a relative command
	Working     string
	TasksMax    int
	StopTimeout time.Duration
	Opts        *enginetypes.VirtualizationCreateOptions
	Resource    *engine.VirtualizationResource
}

// argv renders the systemd-run command that starts the unit.
func (u *unit) argv() []string {
	argv := []string{
		"systemd-run",
		"--unit=" + unitName(u.ID),
		"--slice=" + sliceName(u.Podname),
		"-p", "Description=" + u.description(),
		"-p", "RemainAfterExit=yes",
		"-p", "SyslogIdentifier=" + syslogIdentifier,
	}
	if u.User != "" {
		argv = append(argv, "-p", "User="+u.User)
	}
	if u.Working != "" {
		argv = append(argv, "-p", "WorkingDirectory="+u.Working)
	}
	if u.Root != "" {
		argv = append(argv, "-p", "RootDirectory="+u.Root)
	}
	for _, env := range u.Opts.Env {
		argv = append(argv, "-p", "Environment="+systemdEnv(env))
	}
	for _, property := range properties(u.Resource, u.TasksMax) {
		argv = append(argv, "-p", property)
	}
	for _, property := range bindPaths(u.Resource.Volumes, u.Opts.Env) {
		argv = append(argv, "-p", property)
	}
	if policy := restartPolicy(u.Opts.Restart); policy != "" {
		argv = append(argv, "-p", "Restart="+policy)
	}
	if u.StopTimeout > 0 {
		argv = append(argv, "-p", "TimeoutStopSec="+strconv.FormatInt(int64(u.StopTimeout.Seconds()), 10))
	}
	return append(append(argv, "--"), u.command()...)
}

// command makes ExecStart absolute, which systemd requires; a relative one resolves
// against the unit's own root, or against the bundle when there is none.
func (u *unit) command() []string {
	if len(u.Opts.Cmd) == 0 || filepath.IsAbs(u.Opts.Cmd[0]) {
		return u.Opts.Cmd
	}
	base := "/"
	if u.Root == "" {
		base = u.Bundle
	}
	return slices.Concat([]string{filepath.Join(base, u.Opts.Cmd[0])}, u.Opts.Cmd[1:])
}

func (u *unit) description() string {
	appname, entrypoint, _, err := utils.ParseWorkloadName(u.Opts.Name)
	if err != nil {
		return u.Opts.Name
	}
	return appname + "/" + entrypoint
}

// properties maps the cpumem and storage plugins' engine params onto cgroup v2 knobs.
func properties(resource *engine.VirtualizationResource, tasksMax int) []string {
	props := []string{}
	if cpus := allowedCPUs(resource); cpus != "" {
		props = append(props, "AllowedCPUs="+cpus)
		if resource.NUMANode != "" {
			props = append(props, "AllowedMemoryNodes="+resource.NUMANode)
		}
		props = append(props, "CPUWeight="+strconv.Itoa(cpuWeight(resource.Quota, resource.Remap)))
	}
	if quota := cpuQuota(resource); quota != "" {
		props = append(props, "CPUQuota="+quota)
	}
	if resource.Memory > 0 {
		props = append(props,
			"MemoryMax="+memoryMax(resource),
			"MemoryLow="+memoryLow(resource),
			"MemorySwapMax=0",
		)
	}
	if tasksMax > 0 {
		props = append(props, "TasksMax="+strconv.Itoa(tasksMax))
	}
	return append(props, throttles(resource.IOPSOptions)...)
}

// updateProperties spells every knob out, so a realloc that drops one shape clears what it set.
func updateProperties(resource *engine.VirtualizationResource) []string {
	props := []string{
		"CPUQuota=" + cpuQuota(resource),
		"AllowedCPUs=" + allowedCPUs(resource),
		"AllowedMemoryNodes=" + resource.NUMANode,
		"CPUWeight=" + strconv.Itoa(cpuWeight(resource.Quota, resource.Remap)),
		"MemoryMax=" + memoryMax(resource),
		"MemoryLow=" + memoryLow(resource),
		"MemorySwapMax=0",
	}
	throttled := throttles(resource.IOPSOptions)
	for _, key := range throttleKeys {
		if !slices.ContainsFunc(throttled, func(p string) bool { return strings.HasPrefix(p, key+"=") }) {
			props = append(props, key+"=")
		}
	}
	return append(props, throttled...)
}

func cpuQuota(resource *engine.VirtualizationResource) string {
	if resource.Quota <= 0 {
		return ""
	}
	return fmt.Sprintf("%d%%", int64(math.Round(resource.Quota*quotaPercent)))
}

func allowedCPUs(resource *engine.VirtualizationResource) string {
	return strings.Join(slices.Sorted(maps.Keys(resource.CPU)), " ")
}

func memoryMax(resource *engine.VirtualizationResource) string {
	if resource.Memory <= 0 {
		return "infinity"
	}
	return strconv.FormatInt(resource.Memory, 10)
}

// memoryLow mirrors docker's MemoryReservation: half the limit, never under the engine minimum.
func memoryLow(resource *engine.VirtualizationResource) string {
	if resource.Memory <= 0 {
		return "0"
	}
	return strconv.FormatInt(max(resource.Memory/2, minMemory), 10)
}

// cpuWeight mirrors the docker engine's share split: a bound whole core keeps the default weight.
func cpuWeight(quota float64, remap bool) int {
	_, fraction := math.Modf(quota)
	if remap || fraction == 0 {
		return defaultCPUWeight
	}
	return min(max(1, int(math.Round(defaultCPUWeight*fraction))), maxCPUWeight)
}

// bindPaths maps the volume plugin's src:dst[:mode] bindings onto the unit's mount properties.
// A bind needs no RootDirectory, so raw workloads carry them too.
func bindPaths(volumes, env []string) []string {
	lookup := make(map[string]string, len(env))
	for _, entry := range env {
		if key, value, ok := strings.Cut(entry, "="); ok {
			lookup[key] = value
		}
	}

	props := []string{}
	for _, volume := range volumes {
		parts := strings.Split(os.Expand(volume, func(key string) string { return lookup[key] }), ":")
		if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
			continue
		}
		property := "BindPaths="
		if len(parts) > 2 && parts[2] == readOnlyMode {
			property = "BindReadOnlyPaths="
		}
		props = append(props, property+parts[0]+":"+parts[1])
	}
	return props
}

// bindSources lists the read-write host paths the unit binds; docker creates a missing bind source.
func bindSources(volumes, env []string) []string {
	sources := []string{}
	for _, property := range bindPaths(volumes, env) {
		spec, ok := strings.CutPrefix(property, "BindPaths=")
		if !ok {
			continue
		}
		source, _, _ := strings.Cut(spec, ":")
		sources = append(sources, source)
	}
	return sources
}

func throttles(options map[string]string) []string {
	properties := []string{}
	for _, device := range slices.Sorted(maps.Keys(options)) {
		rates := strings.Split(options[device], ":")
		for len(rates) < len(throttleKeys) {
			rates = append(rates, "0")
		}
		for i, key := range throttleKeys {
			if rate := parseRate(rates[i]); rate > 0 {
				properties = append(properties, fmt.Sprintf("%s=%s %d", key, device, rate))
			}
		}
	}
	return properties
}

func parseRate(rate string) int64 {
	parsed, err := utils.ParseRAMInHuman(rate)
	if err != nil || parsed < 0 {
		return 0
	}
	return parsed
}
