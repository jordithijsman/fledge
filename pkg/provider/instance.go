package provider

import (
	"github.com/virtual-kubelet/virtual-kubelet/node/api"
	"io"
	corev1 "k8s.io/api/core/v1"
	"syscall"
)

type Instance struct {
	ID      string
	Backend Backend
}

func (i *Instance) Status() (corev1.ContainerStatus, error) {
	return i.Backend.GetInstanceStatus(i.ID)
}

func (i *Instance) Create(instance corev1.Container) error {
	return i.Backend.CreateInstance(i.ID, instance)
}

func (i *Instance) Start() error {
	return i.Backend.StartInstance(i.ID)
}

func (i *Instance) Kill(signal syscall.Signal) error {
	return i.Backend.KillInstance(i.ID, signal)
}

func (i *Instance) Delete() error {
	return i.Backend.DeleteInstance(i.ID)
}

func (i *Instance) Logs(opts api.ContainerLogOpts) (io.ReadCloser, error) {
	return i.Backend.GetInstanceLogs(i.ID, opts)
}

func (i *Instance) Run(cmd []string, attach api.AttachIO) error {
	return i.Backend.RunInInstance(i.ID, cmd, attach)
}
