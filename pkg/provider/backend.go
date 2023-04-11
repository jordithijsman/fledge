package provider

import (
	"github.com/virtual-kubelet/virtual-kubelet/node/api"
	"io"
	corev1 "k8s.io/api/core/v1"
	"syscall"
)

const (
	BackendContainerd string = "containerd"
	BackendOsv               = "osv"
)

type BackendOld interface {
	GetContainerName(namespace string, pod corev1.Pod, dc corev1.Container) string
	GetContainerNameAlt(namespace string, podName string, dcName string) string
	CreatePod(pod *corev1.Pod) error
	CreateContainer(namespace string, pod *corev1.Pod, dc *corev1.Container) (string, error)
	UpdatePod(pod *corev1.Pod) error
	DeletePod(pod *corev1.Pod) error
	GetPod(namespace string, name string) (*corev1.Pod, error)
	GetPods() ([]*corev1.Pod, error)
	GetContainerLogs(namespace string, podName string, containerName string, opts api.ContainerLogOpts) (io.ReadCloser, error)
	ShutdownPods()
	PodsChanged() bool
	ResetFlags()
}

type Backend interface {
	GetInstanceStatus(instanceID string) (corev1.ContainerStatus, error)
	CreateInstance(instanceID string, instance corev1.Container) error
	StartInstance(instanceID string) error
	KillInstance(instanceID string, signal syscall.Signal) error
	DeleteInstance(instanceID string) error
	GetInstanceLogs(instanceID string, opts api.ContainerLogOpts) (io.ReadCloser, error)
	RunInInstance(instanceID string, cmd []string, attach api.AttachIO) error
	CreateVolume(volumeID string, volume corev1.Volume) error
	DeleteVolume(volumeID string) error
}
