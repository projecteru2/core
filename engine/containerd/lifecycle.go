package containerd

import (
	"cmp"
	"context"
	"encoding/json"
	"net/url"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/containerd/containerd/api/services/tasks/v1"
	"github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/core/containers"
	"github.com/containerd/containerd/v2/core/runtime/restart"
	"github.com/containerd/containerd/v2/pkg/cio"
	"github.com/containerd/containerd/v2/pkg/oci"
	cerrdefs "github.com/containerd/errdefs"
	"github.com/containerd/typeurl/v2"
	"github.com/moby/sys/signal"
	specs "github.com/opencontainers/runtime-spec/specs-go"

	"github.com/projecteru2/core/engine"
	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/log"
	resourcetypes "github.com/projecteru2/core/resource/types"
	coretypes "github.com/projecteru2/core/types"
)

func (e *Engine) VirtualizationStart(ctx context.Context, ID string) error {
	found, err := e.container(ctx, ID)
	if err != nil {
		return err
	}
	uri, err := url.Parse(logShimURI)
	if err != nil {
		return err
	}
	task, err := found.NewTask(ctx, cio.LogURI(uri))
	if err != nil {
		if !cerrdefs.IsAlreadyExists(err) {
			return err
		}
		if task, err = found.Task(ctx, nil); err != nil {
			return err
		}
	}
	if err = task.Start(ctx); err != nil && !cerrdefs.IsFailedPrecondition(err) {
		return err
	}
	labels, err := found.Labels(ctx)
	if err != nil {
		return err
	}
	return e.setDesiredStatus(ctx, found, labels, client.Running)
}

func (e *Engine) VirtualizationStop(ctx context.Context, ID string, gracefulTimeout time.Duration) error {
	found, err := e.container(ctx, ID)
	if err != nil {
		return err
	}
	labels, err := found.Labels(ctx)
	if err != nil {
		return err
	}
	if err = e.setDesiredStatus(ctx, found, labels, client.Stopped); err != nil {
		return err
	}
	task, err := found.Task(ctx, nil)
	if err != nil {
		if cerrdefs.IsNotFound(err) {
			return nil
		}
		return err
	}
	if err = killTask(ctx, task, stopSignal(labels), e.gracePeriod(gracefulTimeout)); err != nil {
		return err
	}
	_, err = task.Delete(ctx)
	return err
}

func (e *Engine) VirtualizationRemove(ctx context.Context, ID string, _, force bool) error {
	found, err := e.container(ctx, ID)
	if err != nil {
		return err
	}
	labels, err := found.Labels(ctx)
	if err != nil {
		return err
	}
	if err = e.setDesiredStatus(ctx, found, labels, client.Stopped); err != nil {
		return err
	}
	if task, taskErr := found.Task(ctx, nil); taskErr == nil {
		status, statusErr := task.Status(ctx)
		if statusErr == nil && status.Status == client.Running && !force {
			return errors.Wrapf(coretypes.ErrInvaildWorkloadOps, "workload %s is running, stop it first or force the removal", ID)
		}
		if err = killTask(ctx, task, syscall.SIGKILL, 0); err != nil {
			return err
		}
		if _, err = task.Delete(ctx); err != nil && !cerrdefs.IsNotFound(err) {
			return err
		}
	} else if !cerrdefs.IsNotFound(taskErr) {
		return taskErr
	}

	if err = found.Delete(ctx, client.WithSnapshotCleanup); err != nil {
		if cerrdefs.IsNotFound(err) {
			return coretypes.ErrWorkloadNotExists
		}
		return err
	}
	e.discard(ctx, workloadDir(found.ID()))
	return nil
}

func (e *Engine) VirtualizationSuspend(ctx context.Context, ID string) error {
	task, err := e.task(ctx, ID)
	if err != nil {
		return err
	}
	return task.Pause(ctx)
}

func (e *Engine) VirtualizationResume(ctx context.Context, ID string) error {
	task, err := e.task(ctx, ID)
	if err != nil {
		return err
	}
	return task.Resume(ctx)
}

func (e *Engine) VirtualizationInspect(ctx context.Context, ID string) (*enginetypes.VirtualizationInfo, error) {
	found, err := e.container(ctx, ID)
	if err != nil {
		return nil, err
	}
	info, err := found.Info(ctx, client.WithoutRefreshedMetadata)
	if err != nil {
		return nil, err
	}
	r := &enginetypes.VirtualizationInfo{
		ID:       found.ID(),
		Image:    info.Image,
		Labels:   info.Labels,
		Networks: workloadNetworks(info.Labels, e.host),
	}
	if spec, specErr := containerSpec(info); specErr == nil && spec.Process != nil {
		r.User = userString(spec.Process.User)
		r.Env = spec.Process.Env
	}
	resp, err := e.client.TaskService().Get(ctx, &tasks.GetRequest{ContainerID: found.ID()})
	if err != nil {
		if !cerrdefs.IsNotFound(err) {
			return nil, err
		}
		return r, nil
	}
	r.Running = client.ProcessStatus(strings.ToLower(resp.Process.Status.String())) == client.Running
	return r, nil
}

// VirtualizationResize resizes the attach's own pty: ctr forwards its console geometry to the
// task, so setting the task's size behind ctr's back would only be overwritten again.
func (e *Engine) VirtualizationResize(_ context.Context, ID string, height, width uint) error {
	e.mu.Lock()
	running, ok := e.attaches[ID]
	e.mu.Unlock()
	if !ok {
		return errors.Wrap(errAttachNotFound, ID)
	}
	return running.sess.Resize(height, width)
}

func (e *Engine) VirtualizationWait(ctx context.Context, ID, _ string) (*enginetypes.VirtualizationWaitResult, error) {
	exited, err := e.exitWatch(ctx, ID)
	if err != nil {
		return &enginetypes.VirtualizationWaitResult{Message: err.Error(), Code: -1}, err
	}
	select {
	case status := <-exited:
		if err = status.Error(); err != nil {
			return &enginetypes.VirtualizationWaitResult{Message: err.Error(), Code: -1}, err
		}
		return &enginetypes.VirtualizationWaitResult{Code: int64(status.ExitCode())}, nil
	case <-ctx.Done():
		return &enginetypes.VirtualizationWaitResult{Message: ctx.Err().Error(), Code: -1}, ctx.Err()
	}
}

func (e *Engine) VirtualizationUpdateResource(ctx context.Context, ID string, engineParams resourcetypes.Resources) error {
	logger := log.WithFunc("engine.containerd.VirtualizationUpdateResource").WithField("ID", ID)
	resource := &engine.VirtualizationResource{}
	if err := resource.Decode(engineParams); err != nil {
		logger.Errorf(ctx, err, "failed to parse engine args %+v", engineParams)
		return err
	}
	if resource.Memory > 0 && resource.Memory < minMemory || resource.Memory < 0 {
		return coretypes.ErrInvaildMemory
	}

	found, err := e.container(ctx, ID)
	if err != nil {
		return err
	}
	devices, err := e.resolveThrottles(ctx, resource.IOPSOptions)
	if err != nil {
		return err
	}
	limits := resourceSpec(resource, &RawArgs{}, devices)
	// the stored spec is what a restart replays, so both it and the live task are updated
	if err = found.Update(ctx, withSpecResources(limits)); err != nil {
		return err
	}
	task, err := found.Task(ctx, nil)
	if err != nil {
		if cerrdefs.IsNotFound(err) {
			return nil
		}
		return err
	}
	return task.Update(ctx, client.WithResources(limits))
}

// exitWatch prefers the watch an attach registered: `ctr tasks attach` deletes the task when it
// ends, so by the time core waits, the task itself may already be gone.
func (e *Engine) exitWatch(ctx context.Context, ID string) (<-chan client.ExitStatus, error) {
	e.mu.Lock()
	running, ok := e.attaches[ID]
	delete(e.attaches, ID)
	e.mu.Unlock()
	if ok {
		return running.exited, nil
	}
	task, err := e.task(ctx, ID)
	if err != nil {
		return nil, err
	}
	return task.Wait(ctx)
}

// task loads the workload's running task; a workload without one cannot answer.
func (e *Engine) task(ctx context.Context, ID string) (client.Task, error) {
	found, err := e.container(ctx, ID)
	if err != nil {
		return nil, err
	}
	task, err := found.Task(ctx, nil)
	if err != nil && cerrdefs.IsNotFound(err) {
		return nil, errors.Wrapf(coretypes.ErrWorkloadNotExists, "workload %s has no task", ID)
	}
	return task, err
}

// setDesiredStatus tells the restart plugin what core wants; without a policy it does not watch.
func (e *Engine) setDesiredStatus(ctx context.Context, found client.Container, labels map[string]string, status client.ProcessStatus) error {
	if labels[restart.PolicyLabel] == "" || labels[restart.StatusLabel] == string(status) {
		return nil
	}
	_, err := found.SetLabels(ctx, map[string]string{restart.StatusLabel: string(status)})
	return err
}

// containerSpec decodes the spec the container record already carries.
func (e *Engine) gracePeriod(gracefulTimeout time.Duration) time.Duration {
	if gracefulTimeout < 0 {
		return cmp.Or(e.config.Containerd.StopTimeout, defaultStopTimeout)
	}
	return gracefulTimeout
}

func containerSpec(info containers.Container) (*oci.Spec, error) {
	spec := &oci.Spec{}
	if err := json.Unmarshal(info.Spec.GetValue(), spec); err != nil {
		return nil, err
	}
	return spec, nil
}

func withSpecResources(limits *specs.LinuxResources) client.UpdateContainerOpts {
	return func(_ context.Context, _ *client.Client, c *containers.Container) error {
		spec := &oci.Spec{}
		if err := json.Unmarshal(c.Spec.GetValue(), spec); err != nil {
			return err
		}
		if spec.Linux == nil {
			spec.Linux = &specs.Linux{}
		}
		if spec.Linux.Resources == nil {
			spec.Linux.Resources = &specs.LinuxResources{}
		}
		spec.Linux.Resources.CPU = limits.CPU
		spec.Linux.Resources.Memory = limits.Memory
		spec.Linux.Resources.BlockIO = limits.BlockIO
		updated, err := typeurl.MarshalAny(spec)
		if err != nil {
			return err
		}
		c.Spec = updated
		return nil
	}
}

func killSignal(stop syscall.Signal, graceful time.Duration) syscall.Signal {
	if graceful == 0 {
		return syscall.SIGKILL
	}
	return stop
}

// stopSignal is what the image asked to be stopped with; containerd stores it at create.
func stopSignal(labels map[string]string) syscall.Signal {
	name, ok := labels[client.StopSignalLabel]
	if !ok {
		return syscall.SIGTERM
	}
	parsed, err := signal.ParseSignal(name)
	if err != nil {
		return syscall.SIGTERM
	}
	return parsed
}

func killTask(ctx context.Context, task client.Task, stop syscall.Signal, graceful time.Duration) error {
	exited, err := task.Wait(ctx)
	if err != nil {
		return err
	}
	if err = task.Kill(ctx, killSignal(stop, graceful), client.WithKillAll); err != nil && !cerrdefs.IsNotFound(err) {
		return err
	}
	if graceful == 0 {
		return waitExit(ctx, exited)
	}
	timer := time.NewTimer(graceful)
	defer timer.Stop()
	select {
	case <-exited:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
	}
	if err = task.Kill(ctx, syscall.SIGKILL, client.WithKillAll); err != nil && !cerrdefs.IsNotFound(err) {
		return err
	}
	return waitExit(ctx, exited)
}

func waitExit(ctx context.Context, exited <-chan client.ExitStatus) error {
	select {
	case <-exited:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func workloadNetworks(labels map[string]string, host string) map[string]string {
	networks := map[string]string{}
	for key, value := range labels {
		if name, ok := strings.CutPrefix(key, networkLabelPrefix); ok {
			networks[name] = value
		}
	}
	if len(networks) == 0 {
		networks[hostNetwork] = host
	}
	return networks
}

func userString(user specs.User) string {
	if user.UID == 0 && user.GID == 0 {
		return ""
	}
	return strconv.FormatUint(uint64(user.UID), 10) + ":" + strconv.FormatUint(uint64(user.GID), 10)
}
