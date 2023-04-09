package provider

import (
	corev1 "k8s.io/api/core/v1"
	"os"
)

type DummyBackend struct {
	config Config
}

func NewDummyBackend(cfg Config) (*DummyBackend, error) {
	b := &DummyBackend{config: cfg}
	return b, nil
}

//func (b *DummyBackend) GetContainerName(namespace string, pod v1.Pod, dc v1.Instance) string {
//	return "dummy"
//}
//
//func (b *DummyBackend) GetContainerNameAlt(namespace string, podName string, dcName string) string {
//	return "dummy"
//}
//
//func (b *DummyBackend) CreatePod(pod *v1.Pod) error {
//	return nil
//}
//
//func (b *DummyBackend) CreateContainer(namespace string, pod *v1.Pod, dc *v1.Instance) (string, error) {
//	return "", nil
//}
//
//func (b *DummyBackend) UpdatePod(pod *v1.Pod) error {
//	return nil
//}
//
//func (b *DummyBackend) DeletePod(pod *v1.Pod) error {
//	return nil
//}
//
//func (b *DummyBackend) GetPod(namespace string, name string) (*v1.Pod, error) {
//	return nil, nil
//}
//
//func (b *DummyBackend) GetPods() ([]*v1.Pod, error) {
//	return make([]*v1.Pod, 0), nil
//}
//
//func (b *DummyBackend) GetContainerLogs(namespace string, podName string, containerName string, opts api.ContainerLogOpts) (io.ReadCloser, error) {
//	r := io.NopCloser(strings.NewReader("dummy"))
//	return r, nil
//}
//
//func (b *DummyBackend) ShutdownPods() {
//}
//
//func (b *DummyBackend) PodsChanged() bool {
//	return false
//}
//
//func (b *DummyBackend) ResetFlags() {
//}

func (b *DummyBackend) Status(instanceID string) (corev1.ContainerStatus, error) {
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

func (b *DummyBackend) Create(instanceID string, instance corev1.Container) error {
	return nil
}

func (b *DummyBackend) Start(instanceID string) error {
	return nil
}

func (b *DummyBackend) Kill(instanceID string, signal os.Signal) error {
	return nil
}

func (b *DummyBackend) Delete(instanceID string) error {
	return nil
}

// Ensure interface is implemented
var _ Backend = (*DummyBackend)(nil)
