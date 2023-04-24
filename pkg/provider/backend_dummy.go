package provider

import (
	"io"
	"strings"
	"syscall"

	"github.com/virtual-kubelet/virtual-kubelet/node/api"
	corev1 "k8s.io/api/core/v1"
)

type DummyBackend struct {
	config Config
}

func NewDummyBackend(cfg Config) (*DummyBackend, error) {
	b := &DummyBackend{config: cfg}
	return b, nil
}

func (b *DummyBackend) GetInstanceStatus(instanceID string) (corev1.ContainerStatus, error) {
	dummyStatus := corev1.ContainerStatus{
		Name: "dummy",
		State: corev1.ContainerState{
			Terminated: &corev1.ContainerStateTerminated{
				ExitCode:    0,
				Message:     "This container is run by a dummy backend which does absolutely nothing.",
				ContainerID: instanceID,
			},
		},
	}
	return dummyStatus, nil
}

func (b *DummyBackend) CreateInstance(instanceID string, instance corev1.Container) error {
	return nil
}

func (b *DummyBackend) StartInstance(instanceID string) error {
	return nil
}

func (b *DummyBackend) UpdateInstance(instanceID string, instance corev1.Container) error {
	return nil
}

func (b *DummyBackend) KillInstance(instanceID string, signal syscall.Signal) error {
	return nil
}

func (b *DummyBackend) DeleteInstance(instanceID string) error {
	return nil
}

func (b *DummyBackend) GetInstanceLogs(instanceID string, opts api.ContainerLogOpts) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("")), nil
}

func (b *DummyBackend) RunInInstance(instanceID string, cmd []string, attach api.AttachIO) error {
	return nil
}

func (b *DummyBackend) CreateVolume(volumeID string, volume corev1.Volume) error {
	return nil
}

func (b *DummyBackend) UpdateVolume(volumeID string, volume corev1.Volume) error {
	return nil
}

func (b *DummyBackend) DeleteVolume(volumeID string) error {
	return nil
}

// Ensure interface is implemented
var _ Backend = (*DummyBackend)(nil)
