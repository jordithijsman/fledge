package provider

import (
	"github.com/virtual-kubelet/virtual-kubelet/node/api"
	"io"
	corev1 "k8s.io/api/core/v1"
	"os"
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
	Status(instanceID string) (corev1.ContainerStatus, error)
	Create(instanceID string, instance corev1.Container) error
	Start(instanceID string) error
	Kill(instanceID string, signal os.Signal) error
	Delete(instanceID string) error
}
