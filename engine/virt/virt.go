package virt

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"strconv"
	"strings"
	"time"

	"github.com/projecteru2/core/cluster"
	"github.com/projecteru2/core/engine"
	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/log"
	coresource "github.com/projecteru2/core/source"
	"github.com/projecteru2/core/types"
	coretypes "github.com/projecteru2/core/types"
	virtapi "github.com/projecteru2/libyavirt/client"
	virttypes "github.com/projecteru2/libyavirt/types"
)

const (
	// HTTPPrefixKey indicate http yavirtd
	HTTPPrefixKey = "virt://"
	// GRPCPrefixKey indicates grpc yavirtd
	GRPCPrefixKey = "virt-grpc://"
	// DmiUUIDKey indicates the key within deploy info.
	DmiUUIDKey = "DMIUUID"
	// ImageUserKey indicates the image's owner
	ImageUserKey = "ImageUser"
)

// Virt implements the core engine.API interface.
type Virt struct {
	client virtapi.Client
	config coretypes.Config
}

// MakeClient makes a virt. client which wraps yavirt API client.
func MakeClient(ctx context.Context, config coretypes.Config, nodename, endpoint, ca, cert, key string) (engine.API, error) {
	var uri string
	switch {
	case strings.HasPrefix(endpoint, HTTPPrefixKey):
		uri = fmt.Sprintf("http://%s/%s", strings.TrimPrefix(endpoint, HTTPPrefixKey), config.Virt.APIVersion)
	case strings.HasPrefix(endpoint, GRPCPrefixKey):
		uri = "grpc://" + strings.TrimPrefix(endpoint, GRPCPrefixKey)
	default:
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
		NCPU:         resp.CPU,
		MemTotal:     resp.Mem,
		StorageTotal: resp.Storage,
	}, nil
}

// Execute executes a command in vm
func (v *Virt) Execute(ctx context.Context, ID string, config *enginetypes.ExecConfig) (pid string, stdout, stderr io.ReadCloser, stdin io.WriteCloser, err error) {
	if config.Tty {
		flags := virttypes.AttachGuestFlags{Safe: true, Force: true}
		stream, err := v.client.AttachGuest(ctx, ID, config.Cmd, flags)
		if err != nil {
			return "", nil, nil, nil, err
		}
		return "", ioutil.NopCloser(stream), nil, stream, nil

	}
	msg, err := v.client.ExecuteGuest(ctx, ID, config.Cmd)
	return strconv.Itoa(msg.Pid), ioutil.NopCloser(bytes.NewReader(msg.Data)), nil, nil, err
}

// ExecExitCode get return code of a specific execution.
func (v *Virt) ExecExitCode(ctx context.Context, ID, pid string) (code int, err error) {
	intPid, err := strconv.Atoi(pid)
	if err != nil {
		return -1, err
	}
	code, err = v.client.ExecExitCode(ctx, ID, intPid)
	if err != nil {
		return -1, err
	}
	return
}

// ExecResize resize exec tty
func (v *Virt) ExecResize(ctx context.Context, ID, pid string, height, width uint) (err error) {
	return v.client.ResizeConsoleWindow(ctx, ID, height, width)
}

// NetworkConnect connects to a network.
func (v *Virt) NetworkConnect(ctx context.Context, network, target, ipv4, ipv6 string) (cidrs []string, err error) {
	req := virttypes.ConnectNetworkReq{
		Network: network,
		IPv4:    ipv4,
	}
	req.ID = target

	var cidr string
	if cidr, err = v.client.ConnectNetwork(ctx, req); err != nil {
		return
	}

	cidrs = append(cidrs, cidr)

	return
}

// NetworkDisconnect disconnects from one network.
func (v *Virt) NetworkDisconnect(ctx context.Context, network, target string, force bool) (err error) {
	var req virttypes.DisconnectNetworkReq
	req.Network = network
	req.ID = target

	_, err = v.client.DisconnectNetwork(ctx, req)

	return
}

// NetworkList lists all networks.
func (v *Virt) NetworkList(ctx context.Context, drivers []string) (nets []*enginetypes.Network, err error) {
	networks, err := v.client.NetworkList(ctx, drivers)
	if err != nil {
		return nil, err
	}

	for _, network := range networks {
		nets = append(nets, &enginetypes.Network{
			Name:    network.Name,
			Subnets: network.Subnets,
		})
	}
	return
}

// BuildRefs builds references, it's not necessary for virt. presently.
func (v *Virt) BuildRefs(ctx context.Context, name string, tags []string) (refs []string) {
	log.Warnf(ctx, "BuildRefs does not implement")
	return
}

// BuildContent builds content, the use of it is similar to BuildRefs.
func (v *Virt) BuildContent(ctx context.Context, scm coresource.Source, opts *enginetypes.BuildContentOptions) (string, io.Reader, error) {
	return "", nil, fmt.Errorf("BuildContent does not implement")
}

// VirtualizationCreate creates a guest.
func (v *Virt) VirtualizationCreate(ctx context.Context, opts *enginetypes.VirtualizationCreateOptions) (guest *enginetypes.VirtualizationCreated, err error) {
	vols, err := v.parseVolumes(opts.Volumes)
	if err != nil {
		return nil, err
	}

	req := virttypes.CreateGuestReq{
		CPU:        int(opts.Quota),
		Mem:        opts.Memory,
		ImageName:  opts.Image,
		ImageUser:  opts.Labels[ImageUserKey],
		Volumes:    vols,
		Labels:     opts.Labels,
		AncestorID: opts.AncestorWorkloadID,
		DmiUUID:    opts.Labels[DmiUUIDKey],
	}

	var resp virttypes.Guest
	if resp, err = v.client.CreateGuest(ctx, req); err != nil {
		return nil, err
	}

	return &enginetypes.VirtualizationCreated{ID: resp.ID, Name: opts.Name}, nil
}

// VirtualizationResourceRemap .
func (v *Virt) VirtualizationResourceRemap(ctx context.Context, opts *enginetypes.VirtualizationRemapOptions) (ch <-chan enginetypes.VirtualizationRemapMessage, err error) {
	err = types.ErrEngineNotImplemented
	return
}

// VirtualizationCopyTo copies one.
func (v *Virt) VirtualizationCopyTo(ctx context.Context, ID, dest string, content []byte, uid, gid int, mode int64) error {
	return v.client.CopyToGuest(ctx, ID, dest, bytes.NewReader(content), true, true)
}

// VirtualizationStart boots a guest.
func (v *Virt) VirtualizationStart(ctx context.Context, ID string) (err error) {
	_, err = v.client.StartGuest(ctx, ID)
	return
}

// VirtualizationStop stops it.
func (v *Virt) VirtualizationStop(ctx context.Context, ID string, gracefulTimeout time.Duration) (err error) {
	_, err = v.client.StopGuest(ctx, ID, gracefulTimeout == 0)
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

	content, err := json.Marshal(coretypes.LabelMeta{Publish: []string{"PORT"}})
	if err != nil {
		return nil, err
	}

	return &enginetypes.VirtualizationInfo{
		ID:       guest.ID,
		Image:    guest.ImageName,
		Running:  guest.Status == "running",
		Networks: guest.Networks,
		Labels:   map[string]string{cluster.LabelMeta: string(content), cluster.ERUMark: "1"},
	}, nil
}

// VirtualizationLogs streams a specific guest's log.
func (v *Virt) VirtualizationLogs(ctx context.Context, opts *enginetypes.VirtualizationLogStreamOptions) (stdout io.ReadCloser, stderr io.ReadCloser, err error) {
	return nil, nil, fmt.Errorf("VirtualizationLogs does not implement")
}

// VirtualizationAttach attaches something to a guest.
func (v *Virt) VirtualizationAttach(ctx context.Context, ID string, stream, stdin bool) (io.ReadCloser, io.ReadCloser, io.WriteCloser, error) {
	return nil, nil, nil, fmt.Errorf("VirtualizationAttach does not implement")
}

// VirtualizationResize resized window size
func (v *Virt) VirtualizationResize(ctx context.Context, ID string, height, width uint) error {
	return v.client.ResizeConsoleWindow(ctx, ID, height, width)
}

// VirtualizationWait is waiting for a shut-off
func (v *Virt) VirtualizationWait(ctx context.Context, ID, state string) (*enginetypes.VirtualizationWaitResult, error) {
	msg, err := v.client.WaitGuest(ctx, ID, true)
	r := &enginetypes.VirtualizationWaitResult{}
	if err != nil {
		r.Message = err.Error()
		r.Code = -1
		return r, err
	}

	r.Message = msg.Msg
	r.Code = msg.Code
	return r, nil
}

// VirtualizationUpdateResource updates resource.
func (v *Virt) VirtualizationUpdateResource(ctx context.Context, ID string, opts *enginetypes.VirtualizationResource) error {
	vols, err := v.parseVolumes(opts.Volumes)
	if err != nil {
		return err
	}

	args := virttypes.ResizeGuestReq{
		CPU:     int(opts.Quota),
		Mem:     opts.Memory,
		Volumes: vols,
	}
	args.ID = ID

	_, err = v.client.ResizeGuest(ctx, args)
	return err
}

// VirtualizationCopyFrom copies file content from the container.
func (v *Virt) VirtualizationCopyFrom(ctx context.Context, ID, path string) (content []byte, uid, gid int, mode int64, err error) {
	// TODO@zc: virt shall return the properties too
	rd, err := v.client.Cat(ctx, ID, path)
	if err != nil {
		return
	}
	content, err = ioutil.ReadAll(rd)
	return
}

// VirtualizationExecute executes commands in running virtual unit
func (v *Virt) VirtualizationExecute(ctx context.Context, ID string, commands, env []string, workdir string) (io.WriteCloser, io.ReadCloser, error) {
	return nil, nil, fmt.Errorf("VirtualizationExecute not implemented")
}

// ResourceValidate validate resource usage
func (v *Virt) ResourceValidate(ctx context.Context, cpu float64, cpumap map[string]int64, memory, storage int64) error {
	// TODO list all workloads, calculate resource
	return nil
}
