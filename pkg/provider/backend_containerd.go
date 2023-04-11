package provider

import (
	"context"
	"fmt"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/oci"
	"github.com/containerd/containerd/platforms"
	"github.com/google/uuid"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"io"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strings"
	"syscall"

	"github.com/virtual-kubelet/virtual-kubelet/node/api"
	corev1 "k8s.io/api/core/v1"
)

type ContainerdBackend struct {
	config Config

	context context.Context
	client  *containerd.Client
}

func NewContainerdBackend(cfg Config) (*ContainerdBackend, error) {
	client, err := containerd.New(
		"/run/containerd/containerd.sock",
		containerd.WithDefaultNamespace("fledge"),
		containerd.WithDefaultPlatform(platforms.Default()),
	)
	if client == nil {
		return nil, errors.Wrap(err, "containerd")
	}

	b := &ContainerdBackend{
		config:  cfg,
		context: namespaces.WithNamespace(context.Background(), "fledge"),
		client:  client,
	}

	return b, nil
}

func (b *ContainerdBackend) GetInstanceStatus(instanceID string) (corev1.ContainerStatus, error) {
	// Load existing container
	container, err := b.client.LoadContainer(b.context, instanceID)
	if err != nil {
		err = errors.Wrap(err, "containerd")
		return corev1.ContainerStatus{}, err
	}
	// Get container info
	info, err := container.Info(b.context)
	if err != nil {
		err = errors.Wrap(err, "containerd")
		return corev1.ContainerStatus{}, err
	}
	task, err := container.Task(b.context, nil)
	if err != nil {
		err = errors.Wrap(err, "containerd")
		return corev1.ContainerStatus{}, err
	}
	taskStatus, err := task.Status(b.context)
	if err != nil {
		err = errors.Wrap(err, "containerd")
		return corev1.ContainerStatus{}, err
	}
	// Return status (https://pkg.go.dev/github.com/containerd/containerd#ProcessStatus)
	state := corev1.ContainerState{}
	switch taskStatus.Status {
	case containerd.Created:
		state.Waiting = &corev1.ContainerStateWaiting{
			Reason:  "Starting",
			Message: "Starting container",
		}
	case containerd.Running:
		fallthrough
	case containerd.Pausing:
		fallthrough
	case containerd.Paused:
		state.Running = &corev1.ContainerStateRunning{
			StartedAt: metav1.NewTime(taskStatus.ExitTime),
		}
	case containerd.Stopped:
		fallthrough
	case containerd.Unknown:
		state.Terminated = &corev1.ContainerStateTerminated{
			ExitCode:    int32(taskStatus.ExitStatus),
			Reason:      "Stopped",
			Message:     "Container stopped",
			StartedAt:   metav1.NewTime(info.CreatedAt),
			FinishedAt:  metav1.NewTime(taskStatus.ExitTime),
			ContainerID: container.ID(),
		}
	}
	return corev1.ContainerStatus{
		Name:  info.ID,
		State: state,
	}, nil
}

func (b *ContainerdBackend) CreateInstance(instanceID string, instance corev1.Container) error {
	image, err := b.client.GetImage(b.context, instance.Image)
	if (err != nil && instance.ImagePullPolicy == corev1.PullIfNotPresent) || instance.ImagePullPolicy == corev1.PullAlways {
		if instance.ImagePullPolicy == corev1.PullAlways {
			b.client.ImageService().Delete(b.context, instance.Image)
		}
		image, err = b.client.Pull(b.context, instance.Image, containerd.WithPullUnpack)
		if err != nil {
			return errors.Wrap(err, "containerd")
		}
	}

	// Get specification options
	specOpts := []oci.SpecOpts{oci.WithImageConfig(image)}
	// Container.Command
	processArgs := append(instance.Command, instance.Args...)
	if len(processArgs) > 0 {
		specOpts = append(specOpts, oci.WithProcessArgs(processArgs...))
	}
	// Container.WorkingDir
	processCwd := instance.WorkingDir
	if len(processCwd) > 0 {
		specOpts = append(specOpts, oci.WithProcessCwd(processCwd))
	}
	// Container.Ports (TODO)
	// Container.EnvFrom (TODO)
	// Container.Env
	env := make([]string, 0)
	for _, envVar := range instance.Env {
		if envVar.ValueFrom == nil {
			env = append(env, fmt.Sprintf("%q=%q", envVar.Name, envVar.Value))
		}
	}
	specOpts = append(specOpts, oci.WithEnv(env))
	// Container.Resources (TODO)
	// Container.VolumeMounts (TODO)
	// Container.VolumeDevices (TODO)
	// Container.LivenessProbe (TODO)
	// Container.ReadinessProbe (TODO)
	// Container.StartupProbe (TODO)
	// Container.Lifecycle (TODO)
	// Container.TerminationMessagePath (TODO)
	// Container.SecurityContext(TODO)
	// Container.Stdin (TODO)
	// Container.StdinOnce (TODO)
	// Container.TTY (TODO)

	// Create container
	container, err := b.client.NewContainer(
		b.context,
		instanceID,
		containerd.WithImage(image),
		containerd.WithNewSnapshot(fmt.Sprintf("%s_snapshot", instanceID), image),
		containerd.WithNewSpec(specOpts...),
	)
	if err != nil {
		return errors.Wrap(err, "containerd")
	}

	// Create new task
	containerTask, err := container.NewTask(
		b.context,
		cio.NewCreator(cio.WithStdio),
	)
	if err != nil {
		return errors.Wrap(err, "containerd")
	}

	// Wait for task to be created
	_, err = containerTask.Wait(b.context)
	if err != nil {
		return errors.Wrap(err, "containerd")
	}

	return nil
}

func (b *ContainerdBackend) StartInstance(instanceID string) error {
	// Load existing container
	container, err := b.client.LoadContainer(b.context, instanceID)
	if err != nil {
		return errors.Wrap(err, "containerd")
	}

	// Load existing task
	task, err := container.Task(b.context, nil)
	if err != nil {
		return errors.Wrap(err, "containerd")
	}

	// Start task
	if err = task.Start(b.context); err != nil {
		return errors.Wrap(err, "containerd")
	}

	return nil
}

func (b *ContainerdBackend) KillInstance(instanceID string, signal syscall.Signal) error {
	// Load existing container
	container, err := b.client.LoadContainer(b.context, instanceID)
	if err != nil {
		return errors.Wrap(err, "containerd")
	}

	// Load existing task
	task, err := container.Task(b.context, nil)
	if err != nil {
		return errors.Wrap(err, "containerd")
	}

	// Kill task
	killOpts := []containerd.KillOpts{containerd.WithKillAll}
	if err = task.Kill(b.context, signal, killOpts...); err != nil {
		return errors.Wrap(err, "containerd")
	}

	return nil
}

func (b *ContainerdBackend) DeleteInstance(instanceID string) error {
	// Load existing container
	container, err := b.client.LoadContainer(b.context, instanceID)
	if err != nil {
		return errors.Wrap(err, "containerd")
	}

	// Delete container
	if err = container.Delete(b.context); err != nil {
		return errors.Wrap(err, "containerd")
	}

	return nil
}

func (b *ContainerdBackend) GetInstanceLogs(instanceID string, opts api.ContainerLogOpts) (io.ReadCloser, error) {
	// TODO: Backing file
	return io.NopCloser(strings.NewReader("")), nil
}

func (b *ContainerdBackend) RunInInstance(instanceID string, cmd []string, attach api.AttachIO) error {
	// Load existing container
	container, err := b.client.LoadContainer(b.context, instanceID)
	if err != nil {
		return errors.Wrap(err, "containerd")
	}

	// Load existing task
	task, err := container.Task(b.context, nil)
	if err != nil {
		return errors.Wrap(err, "containerd")
	}

	// Generate process ID
	execID := fmt.Sprintf("exec-%s", uuid.New().String())

	// Create process
	pSpec := specs.Process{
		Terminal: attach.TTY(),
		Args:     cmd,
	}

	// Create container IO
	cioOpts := []cio.Opt{cio.WithStreams(attach.Stdin(), attach.Stdout(), attach.Stderr())}
	if attach.TTY() {
		cioOpts = append(cioOpts, cio.WithTerminal)
	}
	ioCreator := cio.NewCreator(cioOpts...)

	// Exec process in task
	process, err := task.Exec(b.context, execID, &pSpec, ioCreator)
	if err != nil {
		return errors.Wrap(err, "containerd")
	}
	defer process.Delete(b.context)

	statusC, err := process.Wait(b.context)
	if err != nil {
		return errors.Wrap(err, "containerd")
	}

	if err = process.Start(b.context); err != nil {
		return errors.Wrap(err, "containerd")
	}

	// Get status code
	status := <-statusC
	code, _, err := status.Result()
	if err != nil {
		return errors.Wrap(err, "containerd")
	}
	if code != 0 {
		err = fmt.Errorf("exec failed with exit code %d", code)
		return errors.Wrap(err, "containerd")
	}

	return nil
}

func (b *ContainerdBackend) CreateVolume(volumeID string, volume corev1.Volume) error {
	return nil
}

func (b *ContainerdBackend) DeleteVolume(volumeID string) error {
	return nil
}

// Ensure interface is implemented
var _ Backend = (*ContainerdBackend)(nil)
