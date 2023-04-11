package provider

import (
	"io"
	"strings"
	"syscall"

	"github.com/virtual-kubelet/virtual-kubelet/node/api"
	corev1 "k8s.io/api/core/v1"
)

type OSvBackend struct {
	config Config
}

func NewOSvBackend(cfg Config) (*OSvBackend, error) {
	b := &OSvBackend{config: cfg}
	return b, nil
}

func (b *OSvBackend) GetInstanceStatus(instanceID string) (corev1.ContainerStatus, error) {
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

func (b *OSvBackend) CreateInstance(instanceID string, instance corev1.Container) error {
	return nil
}

func (b *OSvBackend) StartInstance(instanceID string) error {
	return nil
}

func (b *OSvBackend) KillInstance(instanceID string, signal syscall.Signal) error {
	return nil
}

func (b *OSvBackend) DeleteInstance(instanceID string) error {
	return nil
}

func (b *OSvBackend) GetInstanceLogs(instanceID string, opts api.ContainerLogOpts) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("")), nil
}

func (b *OSvBackend) RunInInstance(instanceID string, cmd []string, attach api.AttachIO) error {
	return nil
}

func (b *OSvBackend) CreateVolume(volumeID string, volume corev1.Volume) error {
	return nil
}

func (b *OSvBackend) DeleteVolume(volumeID string) error {
	return nil
}

// Ensure interface is implemented
var _ Backend = (*OSvBackend)(nil)
