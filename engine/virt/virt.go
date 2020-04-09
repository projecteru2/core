package virt

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"strings"

	log "github.com/sirupsen/logrus"

	virtapi "github.com/projecteru2/libyavirt/client"
	virttypes "github.com/projecteru2/libyavirt/types"

	"github.com/projecteru2/core/cluster"
	"github.com/projecteru2/core/engine"
	enginetypes "github.com/projecteru2/core/engine/types"
	coresource "github.com/projecteru2/core/source"
	coretypes "github.com/projecteru2/core/types"
)

const (
	// HTTPPrefixKey indicate http yavirtd
	HTTPPrefixKey = "virt://"
	// GRPCPrefixKey indicates grpc yavirtd
	GRPCPrefixKey = "virt-grpc://"
	// DmiUUIDKey indicates the key within deploy info.
	DmiUUIDKey = "DMIUUID"
)

// Virt implements the core engine.API interface.
type Virt struct {
	client virtapi.Client
	config coretypes.Config
}

// MakeClient makes a virt. client which wraps yavirt API client.
func MakeClient(ctx context.Context, config coretypes.Config, nodename, endpoint, ca, cert, key string) (engine.API, error) {
	var uri string
	if strings.HasPrefix(endpoint, HTTPPrefixKey) {
		uri = fmt.Sprintf("http://%s/%s", strings.TrimLeft(endpoint, HTTPPrefixKey), config.Virt.APIVersion)
	} else if strings.HasPrefix(endpoint, GRPCPrefixKey) {
		uri = "grpc://" + strings.TrimLeft(endpoint, GRPCPrefixKey)
	} else {
		return nil, fmt.Errorf("invalid endpoint: %s", endpoint)
	}

	cli, err := virtapi.New(uri)
	if err != nil {
		return nil, err
	}
	return &Virt{cli, config}, nil
}

// Info shows a connected node's information.
func (v *Virt) Info(ctx context.Context) (*enginetypes.Info, error) {
	resp, err := v.client.Info(ctx)
	if err != nil {
		return nil, err
	}

	return &enginetypes.Info{
		ID:           resp.ID,
		NCPU:         resp.Cpu,
		MemTotal:     resp.Mem,
		StorageTotal: resp.Storage,
	}, nil
}

// ExecCreate creates an execution.
func (v *Virt) ExecCreate(ctx context.Context, target string, config *enginetypes.ExecConfig) (id string, err error) {
	return "", fmt.Errorf("ExecCreate does not implement")
}

// ExecAttach executes an attachment.
func (v *Virt) ExecAttach(ctx context.Context, execID string, tty bool) (io.ReadCloser, io.WriteCloser, error) {
	return nil, nil, fmt.Errorf("ExecAttach does not implement")
}

// Execute executes a command in vm
func (v *Virt) Execute(ctx context.Context, target string, config *enginetypes.ExecConfig) (_ string, outputStream io.ReadCloser, inputStream io.WriteCloser, err error) {
	if config.Tty {
		flags := virttypes.AttachGuestFlags{Safe: true, Force: true}
		stream, err := v.client.AttachGuest(ctx, target, flags)
		if err != nil {
			return "", nil, nil, err
		}
		return "", ioutil.NopCloser(stream), stream, nil

	}

	msg, err := v.client.ExecuteGuest(ctx, target, config.Cmd)
	return "", ioutil.NopCloser(bytes.NewReader(msg.Data)), nil, err

}

// ExecExitCode gets return code of a specific execution.
func (v *Virt) ExecExitCode(ctx context.Context, execID string) (code int, err error) {
	return 0, nil
}

// ExecResize resize exec tty
func (v *Virt) ExecResize(ctx context.Context, execID string, height, width uint) (err error) {
	return nil
}

// NetworkConnect connects to a network.
func (v *Virt) NetworkConnect(ctx context.Context, network, target, ipv4, ipv6 string) (err error) {
	log.Warnf("NetworkConnect does not implement")
	return
}

// NetworkDisconnect disconnects from one network.
func (v *Virt) NetworkDisconnect(ctx context.Context, network, target string, force bool) (err error) {
	log.Warnf("NetworkDisconnect does not implement")
	return
}

// NetworkList lists all of networks.
func (v *Virt) NetworkList(ctx context.Context, drivers []string) (nets []*enginetypes.Network, err error) {
	log.Warnf("NetworkList does not implement")
	return
}

// BuildRefs builds references, it's not necessary for virt. presently.
func (v *Virt) BuildRefs(ctx context.Context, name string, tags []string) (refs []string) {
	log.Warnf("BuildRefs does not implement")
	return
}

// BuildContent builds content, the use of it is similar to BuildRefs.
func (v *Virt) BuildContent(ctx context.Context, scm coresource.Source, opts *enginetypes.BuildOptions) (string, io.Reader, error) {
	return "", nil, fmt.Errorf("BuildContent does not implement")
}

// VirtualizationCreate creates a guest.
func (v *Virt) VirtualizationCreate(ctx context.Context, opts *enginetypes.VirtualizationCreateOptions) (guest *enginetypes.VirtualizationCreated, err error) {
	vols, err := v.parseVolumes(opts.Volumes)
	if err != nil {
		return nil, err
	}

	stor := MinVirtStorage
	for _, cap := range vols {
		stor += cap
	}
	if opts.Storage < stor {
		return nil, coretypes.NewDetailedErr(coretypes.ErrInsufficientStorage,
			fmt.Sprintf("specify at least %d bytes for the storage", stor))
	}

	req := virttypes.CreateGuestReq{
		Cpu:       int(opts.Quota),
		Mem:       opts.Memory,
		ImageName: opts.Image,
		Volumes:   vols,
	}

	if dmiUUID, exists := opts.Labels[DmiUUIDKey]; exists {
		req.DmiUuid = dmiUUID
	}

	var resp virttypes.Guest
	if resp, err = v.client.CreateGuest(ctx, req); err != nil {
		return nil, err
	}

	return &enginetypes.VirtualizationCreated{ID: resp.ID, Name: opts.Name}, nil
}

// VirtualizationCopyTo copies one.
func (v *Virt) VirtualizationCopyTo(ctx context.Context, ID, target string, content io.Reader, AllowOverwriteDirWithFile, CopyUIDGID bool) (err error) {
	log.Warnf("VirtualizationCopyTo does not implement")
	return
}

// VirtualizationStart boots a guest.
func (v *Virt) VirtualizationStart(ctx context.Context, ID string) (err error) {
	_, err = v.client.StartGuest(ctx, ID)
	return
}

// VirtualizationStop stops it.
func (v *Virt) VirtualizationStop(ctx context.Context, ID string) (err error) {
	_, err = v.client.StopGuest(ctx, ID)
	return
}

// VirtualizationRemove removes a guest.
func (v *Virt) VirtualizationRemove(ctx context.Context, ID string, volumes, force bool) (err error) {
	_, err = v.client.DestroyGuest(ctx, ID, force)
	return
}

// VirtualizationInspect gets a guest.
func (v *Virt) VirtualizationInspect(ctx context.Context, ID string) (*enginetypes.VirtualizationInfo, error) {
	guest, err := v.client.GetGuest(ctx, ID)
	if err != nil {
		return nil, err
	}

	bytes, err := json.Marshal(coretypes.LabelMeta{Publish: []string{"PORT"}})
	if err != nil {
		return nil, err
	}

	return &enginetypes.VirtualizationInfo{
		ID:       guest.ID,
		Image:    guest.ImageName,
		Running:  guest.Status == "running",
		Networks: guest.Networks,
		Labels:   map[string]string{cluster.LabelMeta: string(bytes), cluster.ERUMark: "1"},
	}, nil
}

// VirtualizationLogs streams a specific guest's log.
func (v *Virt) VirtualizationLogs(ctx context.Context, opts *enginetypes.VirtualizationLogStreamOptions) (reader io.ReadCloser, err error) {
	return nil, fmt.Errorf("VirtualizationLogs does not implement")
}

// VirtualizationAttach attaches something to a guest.
func (v *Virt) VirtualizationAttach(ctx context.Context, ID string, stream, stdin bool) (io.ReadCloser, io.WriteCloser, error) {
	return nil, nil, fmt.Errorf("VirtualizationAttach does not implement")
}

// VirtualizationResize resized window size
func (v *Virt) VirtualizationResize(ctx context.Context, ID string, height, width uint) error {
	return fmt.Errorf("VirtualizationResize not implemented")
}

// VirtualizationWait is waiting for a shut-off
func (v *Virt) VirtualizationWait(ctx context.Context, ID, state string) (*enginetypes.VirtualizationWaitResult, error) {
	return nil, fmt.Errorf("VirtualizationWait does not implement")
}

// VirtualizationUpdateResource updates resource.
func (v *Virt) VirtualizationUpdateResource(ctx context.Context, ID string, opts *enginetypes.VirtualizationResource) error {
	vols, err := v.parseVolumes(opts.Volumes)
	if err != nil {
		return err
	}

	args := virttypes.ResizeGuestReq{
		Cpu: int(opts.Quota),
		Mem: opts.Memory,
		Volumes: vols,
	}
	args.ID = ID

	_, err = v.client.ResizeGuest(ctx, args)
	return err
}

// VirtualizationCopyFrom copies from another.
func (v *Virt) VirtualizationCopyFrom(ctx context.Context, ID, path string) (io.ReadCloser, string, error) {
	return nil, "", fmt.Errorf("VirtualizationCopyFrom does not implement")
}

// VirtualizationExecute executes commands in running virtual unit
func (v *Virt) VirtualizationExecute(ctx context.Context, ID string, commands, env []string, workdir string) (io.WriteCloser, io.ReadCloser, error) {
	return nil, nil, fmt.Errorf("VirtualizationExecute not implemented")
}

// ResourceValidate validate resource usage
func (v *Virt) ResourceValidate(ctx context.Context, cpu float64, cpumap map[string]int64, memory, storage int64) error {
	// TODO list all containers, calcuate resource
	return nil
}
