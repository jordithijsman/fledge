package provider

import (
	corev1 "k8s.io/api/core/v1"
	"os"
)

type Instance struct {
	ID      string
	Backend Backend
}

func (i *Instance) Status() (corev1.ContainerStatus, error) {
	return i.Backend.Status(i.ID)
}

func (i *Instance) Create(instance corev1.Container) error {
	return i.Backend.Create(i.ID, instance)
}

func (i *Instance) Start() error {
	return i.Backend.Start(i.ID)
}

func (i *Instance) Kill(signal os.Signal) error {
	return i.Backend.Kill(i.ID, signal)
}

func (i *Instance) Delete() error {
	return i.Backend.Delete(i.ID)
}
