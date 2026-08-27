package process

import (
	"fmt"
	"maps"
	"math"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/projecteru2/core/engine"
	"github.com/projecteru2/core/engine/sshrunner"
	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/utils"
)

const (
	minMemory        = 4 * 1024 * 1024
	defaultCPUWeight = 100
	quotaPercent     = 100
	readOnlyMode     = "ro"
	syslogIdentifier = "eru"
)

var throttleKeys = [...]string{"IOReadIOPSMax", "IOWriteIOPSMax", "IOReadBandwidthMax", "IOWriteBandwidthMax"}

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

func (u *unit) binds() []utils.VolumeBind {
	return utils.ParseVolumeBinds(u.Resource.Volumes, u.Opts.Env)
}

func (u *unit) launcher(dir string) string {
	return "set -- " + sshrunner.Quote(u.staticArgv()) + `
while IFS= read -r p; do
[ -n "$p" ] && set -- "$@" -p "$p"
done < ` + sshrunner.Quote([]string{propsPath(dir)}) + `
exec "$@" -- ` + sshrunner.Quote(u.command())
}

func (u *unit) staticArgv() []string {
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
	if u.TasksMax > 0 {
		argv = append(argv, "-p", "TasksMax="+strconv.Itoa(u.TasksMax))
	}
	for _, property := range bindPaths(u.binds()) {
		argv = append(argv, "-p", property)
	}
	if policy := restartPolicy(u.Opts.Restart); policy != "" {
		argv = append(argv, "-p", "Restart="+policy)
	}
	if u.StopTimeout > 0 {
		argv = append(argv, "-p", "TimeoutStopSec="+strconv.FormatInt(int64(u.StopTimeout.Seconds()), 10))
	}
	return argv
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
	appname, entrypoint, _, _ := utils.ParseWorkloadName(u.Opts.Name)
	return appname + "/" + entrypoint
}

func resourceProperties(resource *engine.VirtualizationResource) []string {
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
	return max(1, int(math.Round(defaultCPUWeight*fraction)))
}

// a bind needs no RootDirectory, so raw workloads carry them too
func bindPaths(binds []utils.VolumeBind) []string {
	props := make([]string, 0, len(binds))
	for _, bind := range binds {
		property := "BindPaths="
		if bind.ReadOnly {
			property = "BindReadOnlyPaths="
		}
		props = append(props, property+bind.Source+":"+bind.Dest)
	}
	return props
}

// bindSources lists the read-write host paths the unit binds; docker creates a missing bind source.
func bindSources(binds []utils.VolumeBind) []string {
	sources := []string{}
	for _, bind := range binds {
		if !bind.ReadOnly {
			sources = append(sources, bind.Source)
		}
	}
	return sources
}

func throttles(options map[string]string) []string {
	properties := []string{}
	for _, device := range slices.Sorted(maps.Keys(options)) {
		rates := strings.Split(options[device], ":")
		for i, key := range throttleKeys {
			if i >= len(rates) {
				break
			}
			if rate := utils.ParseRate(rates[i]); rate > 0 {
				properties = append(properties, fmt.Sprintf("%s=%s %d", key, device, rate))
			}
		}
	}
	return properties
}
