package process

import (
	"fmt"
	"maps"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/projecteru2/core/engine"
	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/utils"
)

const (
	minMemory        = 4 * 1024 * 1024
	defaultCPUWeight = 100
	maxCPUWeight     = 10000
	quotaPercent     = 100
)

var throttleKeys = [...]string{"IOReadIOPSMax", "IOWriteIOPSMax", "IOReadBandwidthMax", "IOWriteBandwidthMax"}

// unit is the transient service that runs one workload.
type unit struct {
	ID       string
	Podname  string
	User     string
	Root     string // RootDirectory, empty for a raw host process
	Working  string
	TasksMax int
	Opts     *enginetypes.VirtualizationCreateOptions
	Resource *engine.VirtualizationResource
}

// argv renders the systemd-run command that starts the unit.
func (u *unit) argv() []string {
	argv := []string{
		"systemd-run",
		"--unit=" + unitName(u.ID),
		"--slice=" + sliceName(u.Podname),
		"--collect",
		"-p", "Description=" + u.description(),
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
	if policy := restartPolicy(u.Opts.Restart); policy != "" {
		argv = append(argv, "-p", "Restart="+policy)
	}
	return append(append(argv, "--"), u.Opts.Cmd...)
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
	switch {
	case len(resource.CPU) > 0:
		props = append(props, "AllowedCPUs="+strings.Join(slices.Sorted(maps.Keys(resource.CPU)), " "))
		if resource.NUMANode != "" {
			props = append(props, "AllowedMemoryNodes="+resource.NUMANode)
		}
		props = append(props, "CPUWeight="+strconv.Itoa(cpuWeight(resource.Quota, resource.Remap)))
	case resource.Quota > 0:
		props = append(props, fmt.Sprintf("CPUQuota=%d%%", int64(math.Round(resource.Quota*quotaPercent))))
	}
	if resource.Memory > 0 {
		props = append(props,
			"MemoryMax="+strconv.FormatInt(resource.Memory, 10),
			"MemoryHigh="+strconv.FormatInt(max(resource.Memory/2, minMemory), 10),
		)
	}
	if tasksMax > 0 {
		props = append(props, "TasksMax="+strconv.Itoa(tasksMax))
	}
	return append(props, throttles(resource.IOPSOptions)...)
}

// cpuWeight mirrors the docker engine's share split: a bound whole core keeps the default weight.
func cpuWeight(quota float64, remap bool) int {
	_, fraction := math.Modf(quota)
	if remap || fraction == 0 {
		return defaultCPUWeight
	}
	return min(max(1, int(math.Round(defaultCPUWeight*fraction))), maxCPUWeight)
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
