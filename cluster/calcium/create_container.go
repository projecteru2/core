package calcium

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	log "github.com/Sirupsen/logrus"
	enginetypes "github.com/docker/engine-api/types"
	enginecontainer "github.com/docker/engine-api/types/container"
	enginenetwork "github.com/docker/engine-api/types/network"
	engineslice "github.com/docker/engine-api/types/strslice"
	"github.com/docker/go-units"
	"gitlab.ricebook.net/platform/core/types"
	"gitlab.ricebook.net/platform/core/utils"
	"golang.org/x/net/context"
)

// Create Container
// Use specs and options to create
// TODO need to call agent's API to create network
func (c *Calcium) CreateContainer(specs *types.Specs, opts *types.DeployOptions) (chan *types.CreateContainerMessage, error) {
	ch := make(chan *types.CreateContainerMessage)

	result, err := c.prepareNodes(opts.Podname, opts.CPUQuota, opts.Count)
	if err != nil {
		return ch, err
	}
	if len(result) == 0 {
		return ch, fmt.Errorf("Not enough resource to create container")
	}

	// check total count in case scheduler error
	totalCount := 0
	for _, cores := range result {
		totalCount = totalCount + len(cores)
	}
	if totalCount != opts.Count {
		return ch, fmt.Errorf("Count mismatch (opt.Count %q, total %q), maybe scheduler error?", opts.Count, totalCount)
	}

	go func() {
		wg := sync.WaitGroup{}
		wg.Add(len(result))

		// do deployment
		for nodename, cpumap := range result {
			go func(nodename string, cpumap []types.CPUMap, opts *types.DeployOptions) {
				defer wg.Done()

				for _, m := range c.doCreateContainer(nodename, cpumap, specs, opts) {
					ch <- m
				}
			}(nodename, cpumap, opts)
		}

		wg.Wait()
		close(ch)
	}()

	return ch, nil
}

func makeCPUMap(nodes []*types.Node) map[string]types.CPUMap {
	r := make(map[string]types.CPUMap)
	for _, node := range nodes {
		r[node.Name] = node.CPU
	}
	return r
}

// Prepare nodes for deployment.
// Later if any error occurs, these nodes can be restored.
func (c *Calcium) prepareNodes(podname string, quota float64, num int) (map[string][]types.CPUMap, error) {
	// TODO use distributed lock on podname instead
	c.Lock()
	defer c.Unlock()

	r := make(map[string][]types.CPUMap)

	nodes, err := c.ListPodNodes(podname)
	if err != nil {
		return r, err
	}

	cpumap := makeCPUMap(nodes)
	// TODO too be simple, just use int type as quota
	r, err = c.scheduler.SelectNodes(cpumap, int(quota), num)
	if err != nil {
		return r, err
	}

	// cpus remained
	// update data to etcd
	// `SelectNodes` reduces count in cpumap
	for _, node := range nodes {
		node.CPU = cpumap[node.Name]
		// ignore error
		c.store.UpdateNode(node)
	}

	return r, err
}

// Pull an image
// Blocks until it finishes.
func doPullImage(node *types.Node, image string) error {
	if image == "" {
		return fmt.Errorf("No image found for version")
	}

	resp, err := node.Engine.ImagePull(context.Background(), image, enginetypes.ImagePullOptions{})
	if err != nil {
		return err
	}
	ensureReaderClosed(resp)
	return nil
}

func (c *Calcium) doCreateContainer(nodename string, cpumap []types.CPUMap, specs *types.Specs, opts *types.DeployOptions) []*types.CreateContainerMessage {
	ms := make([]*types.CreateContainerMessage, len(cpumap))
	node, err := c.GetNode(opts.Podname, nodename)
	if err != nil {
		return ms
	}

	if err := doPullImage(node, opts.Image); err != nil {
		return ms
	}

	for i, quota := range cpumap {
		config, hostConfig, networkConfig, containerName, err := c.makeContainerOptions(quota, specs, opts)
		if err != nil {
			log.Errorf("error when creating CreateContainerOptions, %v", err)
			c.releaseQuota(node, quota)
			continue
		}

		container, err := node.Engine.ContainerCreate(context.Background(), config, hostConfig, networkConfig, containerName)
		if err != nil {
			log.Errorf("error when creating container, %v", err)
			c.releaseQuota(node, quota)
			continue
		}

		err = node.Engine.ContainerStart(context.Background(), container.ID, enginetypes.ContainerStartOptions{})
		if err != nil {
			log.Errorf("error when starting container, %v", err)
			c.releaseQuota(node, quota)
			go node.Engine.ContainerRemove(context.Background(), container.ID, enginetypes.ContainerRemoveOptions{})
			continue
		}

		info, err := node.Engine.ContainerInspect(context.Background(), container.ID)
		if err != nil {
			log.Errorf("error when inspecting container, %v", err)
			c.releaseQuota(node, quota)
			continue
		}

		_, err = c.store.AddContainer(info.ID, opts.Podname, node.Name)
		if err != nil {
			c.releaseQuota(node, quota)
			continue
		}

		ms[i] = &types.CreateContainerMessage{
			Podname:       opts.Podname,
			Nodename:      node.Name,
			ContainerID:   info.ID,
			ContainerName: containerName,
			Success:       true,
			CPU:           quota,
		}
	}

	return ms
}

func (c *Calcium) releaseQuota(node *types.Node, quota types.CPUMap) {
	node.CPU.Add(quota)
	c.store.UpdateNode(node)
}

func (c *Calcium) makeContainerOptions(quota map[string]int, specs *types.Specs, opts *types.DeployOptions) (
	*enginecontainer.Config,
	*enginecontainer.HostConfig,
	*enginenetwork.NetworkingConfig,
	string,
	error) {

	entry, ok := specs.Entrypoints[opts.Entrypoint]
	if !ok {
		return nil, nil, nil, "", fmt.Errorf("Entrypoint %q not found in image %q", opts.Entrypoint, opts.Image)
	}

	// command
	slices := strings.Split(entry.Command, " ")
	starter, needNetwork := "launcher", "network"
	if !opts.Raw {
		if entry.Privileged != "" {
			starter = "launcheroot"
		}
		if len(opts.Networks) == 0 {
			needNetwork = "nonetwork"
		}
		slices = append([]string{fmt.Sprintf("/usr/local/bin/%s", starter), needNetwork}, slices...)
	}
	cmd := engineslice.StrSlice(slices)

	// calculate CPUShares and CPUSet
	// scheduler won't return more than 1 share quota
	// so the smallest share is the share numerator
	shareQuota := 10
	labels := []string{}
	for label, share := range quota {
		labels = append(labels, label)
		if share < shareQuota {
			shareQuota = share
		}
	}
	cpuShares := int64(float64(shareQuota) / float64(10) * float64(1024))
	cpuSetCpus := strings.Join(labels, ",")

	// env
	env := append(opts.Env, fmt.Sprintf("APP_NAME=%s", specs.Appname))
	env = append(env, fmt.Sprintf("ERU_POD=%s", opts.Podname))

	// volumes and binds
	volumes := make(map[string]struct{})
	volumes["/writable-proc/sys"] = struct{}{}

	binds := []string{}
	binds = append(binds, "/proc/sys:/writable-proc/sys:ro")

	// add permdir to container
	if entry.PermDir {
		permDir := filepath.Join("/", specs.Appname, "permdir")
		permDirHost := filepath.Join(c.config.PermDir, specs.Appname)
		volumes[permDir] = struct{}{}

		binds = append(binds, strings.Join([]string{permDirHost, permDir, "rw"}, ":"))
		env = append(env, fmt.Sprintf("ERU_PERMDIR=%s", permDir))
	}

	for _, volume := range specs.Volumes {
		volumes[volume] = struct{}{}
	}

	var mode string
	for hostPath, bind := range specs.Binds {
		if bind.ReadOnly {
			mode = "ro"
		} else {
			mode = "rw"
		}
		binds = append(binds, strings.Join([]string{hostPath, bind.InContainerPath, mode}, ":"))
	}

	// log config
	logConfig := "none"
	if entry.LogConfig == "json-file" {
		logConfig = "json-file"
	}

	// working dir is /:appname if it's not deployed as raw app
	workingDir := "/"
	if !opts.Raw {
		workingDir = "/" + specs.Appname
	}

	// CapAdd and Privileged
	capAdd := []string{}
	if entry.Privileged == "__super__" {
		capAdd = append(capAdd, "SYS_ADMIN")
	}

	// ulimit
	ulimits := []*units.Ulimit{&units.Ulimit{Name: "nofile", Soft: 65535, Hard: 65535}}

	// name
	suffix := utils.RandomString(6)
	containerName := strings.Join([]string{specs.Appname, opts.Entrypoint, suffix}, "_")

	config := &enginecontainer.Config{
		Env:             env,
		Cmd:             cmd,
		Image:           opts.Image,
		Volumes:         volumes,
		WorkingDir:      workingDir,
		NetworkDisabled: false,
		Labels:          make(map[string]string),
	}
	hostConfig := &enginecontainer.HostConfig{
		Binds:         binds,
		LogConfig:     enginecontainer.LogConfig{Type: logConfig},
		NetworkMode:   enginecontainer.NetworkMode(entry.NetworkMode),
		RestartPolicy: enginecontainer.RestartPolicy{Name: entry.RestartPolicy, MaximumRetryCount: 3},
		CapAdd:        engineslice.StrSlice(capAdd),
		ExtraHosts:    entry.ExtraHosts,
		Privileged:    entry.Privileged != "",
		Resources: enginecontainer.Resources{
			CPUShares:  cpuShares,
			CpusetCpus: cpuSetCpus,
			Ulimits:    ulimits,
		},
	}
	// this is empty because we don't use any plugin for Docker
	networkConfig := &enginenetwork.NetworkingConfig{}
	return config, hostConfig, networkConfig, containerName, nil
}
