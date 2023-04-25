package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/oci"
	"github.com/containerd/containerd/platforms"
	gocni "github.com/containerd/go-cni"
	"github.com/containerd/nerdctl/pkg/labels"
	"github.com/google/uuid"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"io"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"os"
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

func (b *ContainerdBackend) GetInstanceStatus(instance *Instance) (corev1.ContainerStatus, error) {
	// Load existing container
	container, err := b.client.LoadContainer(b.context, instance.ID)
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

func (b *ContainerdBackend) CreateInstance(instance *Instance) error {
	// Container.Image
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

	// Get container and specification options
	containerOpts := []containerd.NewContainerOpts{
		containerd.WithImage(image),
		containerd.WithNewSnapshot(fmt.Sprintf("%s_snapshot", instance.ID), image),
	}
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
	// Container.Ports
	portsContainerOpts, err := b.getPortsOpts(instance.Ports)
	if err != nil {
		return errors.Wrap(err, "containerd")
	}
	containerOpts = append(containerOpts, portsContainerOpts...)
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
	// Container.VolumeMounts
	volumeMountsContainerOpts, volumeMountsSpecOpts, err := b.getVolumeMountsOpts(instance.VolumeMounts)
	if err != nil {
		return errors.Wrap(err, "containerd")
	}
	containerOpts = append(containerOpts, volumeMountsContainerOpts...)
	specOpts = append(specOpts, volumeMountsSpecOpts...)
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
	containerOpts = append(containerOpts, containerd.WithNewSpec(specOpts...))
	container, err := b.client.NewContainer(
		b.context,
		instance.ID,
		containerOpts...,
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

func (b *ContainerdBackend) StartInstance(instance *Instance) error {
	// Load existing container
	container, err := b.client.LoadContainer(b.context, instance.ID)
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

func (b *ContainerdBackend) UpdateInstance(instance *Instance) error {
	// TODO: Can we do this more performant?
	if err := b.DeleteInstance(instance); err != nil {
		return err
	}
	return b.CreateInstance(instance)
}

func (b *ContainerdBackend) KillInstance(instance *Instance, signal syscall.Signal) error {
	// Load existing container
	container, err := b.client.LoadContainer(b.context, instance.ID)
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

func (b *ContainerdBackend) DeleteInstance(instance *Instance) error {
	// Load existing container
	container, err := b.client.LoadContainer(b.context, instance.ID)
	if err != nil {
		return errors.Wrap(err, "containerd")
	}

	// Delete container
	if err = container.Delete(b.context); err != nil {
		return errors.Wrap(err, "containerd")
	}

	return nil
}

func (b *ContainerdBackend) GetInstanceLogs(instance *Instance, opts api.ContainerLogOpts) (io.ReadCloser, error) {
	// TODO: Backing file
	return io.NopCloser(strings.NewReader("")), nil
}

func (b *ContainerdBackend) RunInInstance(instance *Instance, cmd []string, attach api.AttachIO) error {
	// Load existing container
	container, err := b.client.LoadContainer(b.context, instance.ID)
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

func (b *ContainerdBackend) getPortsOpts(containerPorts []corev1.ContainerPort) ([]containerd.NewContainerOpts, error) {
	// TODO:  ad-hoc; check nerdctl/cmd/nerdctl/container_run_network.go
	var ports []gocni.PortMapping
	for _, cp := range containerPorts {
		// TODO: Just expose a port for now, do nothing special
		port := gocni.PortMapping{
			HostPort:      cp.HostPort,
			ContainerPort: cp.ContainerPort,
			Protocol:      string(cp.Protocol),
			HostIP:        cp.HostIP,
		}
		ports = append(ports, port)
	}

	portsJson, err := json.Marshal(ports)
	if err != nil {
		return nil, errors.Wrap(err, "containerd")
	}
	portsLabels := map[string]string{labels.Ports: string(portsJson)}
	return []containerd.NewContainerOpts{containerd.WithAdditionalContainerLabels(portsLabels)}, nil
}

func (b *ContainerdBackend) getVolumeMountsOpts(volumeMounts []InstanceVolumeMount) ([]containerd.NewContainerOpts, []oci.SpecOpts, error) {
	var mounts []specs.Mount
	for _, vm := range volumeMounts {
		v := vm.Volume
		switch {
		case v.HostPath != nil:
			switch *v.HostPath.Type {
			case corev1.HostPathDirectoryOrCreate:
				if err := os.MkdirAll(v.HostPath.Path, 0755); err != nil {
					return nil, nil, err
				}
				fallthrough
			case corev1.HostPathDirectory:
			default:
				return nil, nil, errors.Errorf("volumeMount %q has unsupported hostPath.type %q", v.ID, v.HostPath.Type)
			}
			mount := specs.Mount{
				Type:        "none",
				Source:      v.HostPath.Path,
				Destination: vm.MountPath,
				Options:     []string{},
			}
			mounts = append(mounts, mount)
		// TODO: EmptyDir *corev1.EmptyDirVolumeSource
		// TODO: GCEPersistentDisk *corev1.GCEPersistentDiskVolumeSource
		// TODO: AWSElasticBlockStore *corev1.AWSElasticBlockStoreVolumeSource
		// TODO: GitRepo *corev1.GitRepoVolumeSource
		case v.Secret != nil:
		case v.NFS != nil:
		// TODO: ISCSI *ISCSIVolumeSource
		// TODO: Glusterfs *GlusterfsVolumeSource
		// TODO: PersistentVolumeClaim *PersistentVolumeClaimVolumeSource
		// TODO: RBD *RBDVolumeSource
		// TODO: FlexVolume *FlexVolumeSource
		// TODO: Cinder *CinderVolumeSource
		// TODO: CephFS *CephFSVolumeSource
		// TODO: Flocker *FlockerVolumeSource
		// TODO: DownwardAPI *DownwardAPIVolumeSource
		// TODO: FC *FCVolumeSource
		// TODO: AzureFile *AzureFileVolumeSource
		case v.ConfigMap != nil:
		// TODO: VsphereVolume *VsphereVirtualDiskVolumeSource
		// TODO: Quobyte *QuobyteVolumeSource
		// TODO: AzureDisk *AzureDiskVolumeSource
		// TODO: PhotonPersistentDisk *PhotonPersistentDiskVolumeSource
		case v.Projected != nil:
			// TODO: Solve this with another virtio-fs mount?
			for _, s := range v.Projected.Sources {
				switch {
				case s.Secret != nil:
					// TODO
				case s.DownwardAPI != nil:
					// TODO
				case s.ConfigMap != nil:
					// TODO
				case s.ServiceAccountToken != nil:
					// TODO
				}
			}
		// TODO: PortworxVolume *PortworxVolumeSource
		// TODO: ScaleIO *ScaleIOVolumeSource
		// TODO: StorageOS *StorageOSVolumeSource
		// TODO: CSI *CSIVolumeSource
		// TODO: Ephemeral *EphemeralVolumeSource
		default:
			err := errors.Errorf("volumeMount %q has an unsupported type", vm.Name)
			return nil, nil, errors.Wrap(err, "osv")
		}
	}

	mountsJson, err := json.Marshal(mounts)
	if err != nil {
		return nil, nil, errors.Wrap(err, "containerd")
	}
	mountsLabels := map[string]string{labels.Mounts: string(mountsJson)}
	return []containerd.NewContainerOpts{containerd.WithAdditionalContainerLabels(mountsLabels)}, []oci.SpecOpts{oci.WithMounts(mounts)}, nil
}

// Ensure interface is implemented
var _ Backend = (*ContainerdBackend)(nil)
