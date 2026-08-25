package docker

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"text/template"

	"github.com/distribution/reference"
	"github.com/docker/docker/api/types/blkiodev"
	dockercontainer "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/docker/registry"
	"github.com/moby/go-archive"
	"github.com/moby/go-archive/compression"

	corecluster "github.com/projecteru2/core/cluster"
	"github.com/projecteru2/core/engine"
	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/log"
	coretypes "github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

func CreateTarStream(path string) (io.ReadCloser, error) {
	tarOpts := &archive.TarOptions{
		ExcludePatterns: []string{},
		IncludeFiles:    []string{"."},
		Compression:     compression.None,
		NoLchown:        true,
	}
	return archive.TarWithOptions(path, tarOpts)
}

// GetIP returns the host part of a docker daemon endpoint.
func GetIP(ctx context.Context, daemonHost string) string {
	u, err := url.Parse(daemonHost)
	if err != nil {
		log.WithFunc("engine.docker.GetIP").Errorf(ctx, err, "parse daemon host %s", daemonHost)
		return ""
	}
	return u.Hostname()
}

func mergeStream(stream io.ReadCloser) io.Reader {
	outr, outw := io.Pipe()

	go func() {
		defer func() {
			_ = stream.Close()
		}()
		_, err := stdcopy.StdCopy(outw, outw, stream)
		_ = outw.CloseWithError(err)
	}()

	return outr
}

// volume format is docker's src:dst:mode, with an optional size field
func makeMountPaths(ctx context.Context, opts *enginetypes.VirtualizationCreateOptions, resourceOpts *engine.VirtualizationResource) ([]string, map[string]struct{}) {
	binds := []string{}
	volumes := make(map[string]struct{})

	envMap := make(map[string]string, len(opts.Env))
	for _, env := range opts.Env {
		if key, value, ok := strings.Cut(env, "="); ok {
			envMap[key] = value
		}
	}

	for _, path := range resourceOpts.Volumes {
		expanded := os.Expand(path, func(key string) string { return envMap[key] })
		parts := strings.Split(expanded, ":")
		if len(parts) == 2 {
			binds = append(binds, fmt.Sprintf("%s:%s:rw", parts[0], parts[1]))
			volumes[parts[1]] = struct{}{}
		} else if len(parts) >= 3 {
			binds = append(binds, fmt.Sprintf("%s:%s:%s", parts[0], parts[1], parts[2]))
			volumes[parts[1]] = struct{}{}
			if len(parts) == 4 {
				log.WithFunc("engine.docker.makeMountPaths").Warn(ctx, "docker engine does not support volume with size limit")
			}
		}
	}

	return binds, volumes
}

func makeResourceSetting(cpu float64, memory int64, cpuMap map[string]int64, numaNode string, IOPSOptions map[string]string, remap bool) dockercontainer.Resources {
	resource := dockercontainer.Resources{}

	resource.CPUQuota = 0
	resource.CPUShares = defaultCPUShare
	resource.CPUPeriod = corecluster.CPUPeriodBase
	if cpu > 0 {
		resource.CPUQuota = int64(cpu * float64(corecluster.CPUPeriodBase))
	} else if cpu == -1 {
		resource.CPUQuota = -1
	}

	if len(cpuMap) > 0 {
		resource.CpusetCpus = strings.Join(slices.Sorted(maps.Keys(cpuMap)), ",")
		resource.CpusetMems = numaNode

		if remap {
			resource.CPUShares = defaultCPUShare
		} else {
			// bound cpus run without a quota
			resource.CPUQuota = -1
			if _, divpart := math.Modf(cpu); divpart > 0 {
				resource.CPUShares = int64(math.Round(defaultCPUShare * divpart))
			}
		}
	}
	resource.Memory = memory
	resource.MemorySwap = memory
	resource.MemoryReservation = memory / 2
	if memory != 0 && memory/2 < minMemory {
		resource.MemoryReservation = minMemory
	}

	if len(IOPSOptions) > 0 {
		var readIOPSDevices, writeIOPSDevices, readBPSDevices, writeBPSDevices []*blkiodev.ThrottleDevice
		for device, options := range IOPSOptions {
			parts := strings.Split(options, ":")
			for len(parts) < 4 {
				parts = append(parts, "0")
			}
			readIOPSDevices = append(readIOPSDevices, &blkiodev.ThrottleDevice{
				Path: device,
				Rate: parseThrottleRate(parts[0]),
			})
			writeIOPSDevices = append(writeIOPSDevices, &blkiodev.ThrottleDevice{
				Path: device,
				Rate: parseThrottleRate(parts[1]),
			})
			readBPSDevices = append(readBPSDevices, &blkiodev.ThrottleDevice{
				Path: device,
				Rate: parseThrottleRate(parts[2]),
			})
			writeBPSDevices = append(writeBPSDevices, &blkiodev.ThrottleDevice{
				Path: device,
				Rate: parseThrottleRate(parts[3]),
			})
		}
		resource.BlkioDeviceReadIOps = readIOPSDevices
		resource.BlkioDeviceWriteIOps = writeIOPSDevices
		resource.BlkioDeviceReadBps = readBPSDevices
		resource.BlkioDeviceWriteBps = writeBPSDevices
	}

	return resource
}

func parseThrottleRate(s string) uint64 {
	rate, err := utils.ParseRAMInHuman(s)
	if err != nil || rate < 0 {
		return 0
	}
	return uint64(rate)
}

func normalizeImage(image string) string {
	if strings.Contains(image, ":") {
		t := strings.Split(image, ":")
		return t[0]
	}
	return image
}

// See https://github.com/docker/cli/blob/16cccc30f95c8163f0749eba5a2e80b807041342/cli/command/registry.go#L67
func makeEncodedAuthConfigFromRemote(authConfigs map[string]coretypes.AuthConfig, remote string) (string, error) {
	ref, err := reference.ParseNormalizedNamed(remote)
	if err != nil {
		return "", err
	}

	repoInfo, err := registry.ParseRepositoryInfo(ref)
	if err != nil {
		return "", err
	}

	serverAddress := repoInfo.Index.Name
	if authConfig, exists := authConfigs[serverAddress]; exists {
		encodedAuth, encodeErr := encodeAuthToBase64(authConfig)
		if encodeErr != nil {
			return "", encodeErr
		}
		return encodedAuth, nil
	}
	return "dummy", nil
}

// See https://github.com/docker/cli/blob/master/cli/command/registry.go#L41
func encodeAuthToBase64(authConfig coretypes.AuthConfig) (string, error) {
	buf, err := json.Marshal(authConfig) //nolint:gosec // the registry X-Registry-Auth header is defined as this credential payload
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(buf), nil
}

func createImageTag(config coretypes.DockerConfig, appname, tag string) string {
	prefix := strings.Trim(config.Namespace, "/")
	if prefix == "" {
		return fmt.Sprintf("%s/%s:%s", config.Hub, appname, tag)
	}
	return fmt.Sprintf("%s/%s/%s:%s", config.Hub, prefix, appname, tag)
}

func makeCommonPart(build *enginetypes.Build) (string, error) {
	tmpl := template.Must(template.New("common").Parse(commonTmpl))
	out := bytes.Buffer{}
	if err := tmpl.Execute(&out, build); err != nil {
		return "", err
	}
	return out.String(), nil
}

func makeUserPart(opts *enginetypes.BuildContentOptions) (string, error) {
	tmpl := template.Must(template.New("user").Parse(userTmpl))
	out := bytes.Buffer{}
	if err := tmpl.Execute(&out, opts); err != nil {
		return "", err
	}
	return out.String(), nil
}

func makeMainPart(_ *enginetypes.BuildContentOptions, build *enginetypes.Build, from string, commands, copys []string) (string, error) {
	var buildTmpl []string
	common, err := makeCommonPart(build)
	if err != nil {
		return "", err
	}
	buildTmpl = append(buildTmpl, from, common)
	if len(copys) > 0 {
		buildTmpl = append(buildTmpl, copys...)
	}
	if len(commands) > 0 {
		buildTmpl = append(buildTmpl, commands...)
	}
	return strings.Join(buildTmpl, "\n"), nil
}

func recreateDir(path string) error {
	if err := os.RemoveAll(path); err != nil {
		return err
	}
	return os.MkdirAll(path, os.ModeDir)
}

func createDockerfile(dockerfile, buildDir string) (err error) {
	f, err := os.Create(filepath.Clean(filepath.Join(buildDir, "Dockerfile")))
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := f.Close(); err == nil {
			err = closeErr
		}
	}()
	_, err = f.WriteString(dockerfile)
	return err
}

func useCNI(labels map[string]string) bool {
	return labels["cni"] == "1"
}
