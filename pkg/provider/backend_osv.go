package provider

import (
	"encoding/json"
	"fmt"
	"github.com/cloudius-systems/capstan/core"
	"github.com/cloudius-systems/capstan/hypervisor/qemu"
	"github.com/cloudius-systems/capstan/nat"
	"github.com/containerd/containerd/log"
	"github.com/pkg/errors"
	"github.com/regclient/regclient"
	"github.com/regclient/regclient/types"
	"github.com/regclient/regclient/types/ref"
	"gitlab.ilabt.imec.be/fledge/service/pkg/storage"
	"gitlab.ilabt.imec.be/fledge/service/pkg/system"
	"golang.org/x/net/context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	capstan "github.com/cloudius-systems/capstan/util"
	"github.com/virtual-kubelet/virtual-kubelet/node/api"
	corev1 "k8s.io/api/core/v1"
)

type OSvBackend struct {
	config Config

	context context.Context
	repo    *capstan.Repo
}

func NewOSvBackend(cfg Config) (*OSvBackend, error) {
	repo := capstan.NewRepo("")

	b := &OSvBackend{
		config:  cfg,
		context: context.Background(),
		repo:    repo,
	}
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
	// Get image config
	imageConf, err := storage.ImageGetConfig(b.context, instance.Image)
	if err != nil {
		return errors.Wrap(err, "osv")
	}

	// Container.Image
	imagePath := filepath.Join(b.repo.RepoPath(), "fledge", storage.CleanName(instance.Image))
	// Check if the image exists
	_, err = os.Stat(imagePath)
	imageExists := !os.IsNotExist(err)
	// Pull image if required
	if (!imageExists && instance.ImagePullPolicy == corev1.PullIfNotPresent) || instance.ImagePullPolicy == corev1.PullAlways {
		if err = b.pullInstanceImage(instance.Image, imageConf.Hypervisor); err != nil {
			return errors.Wrap(err, "osv")
		}
	}

	// Get hypervisor options
	// Container.Image
	image := b.imageDiskPath(instance.Image, imageConf.Hypervisor)
	// Container.Command
	cmd := strings.Join(append(instance.Command, instance.Args...), " ")
	// Container.WorkingDir (TODO)
	// Container.Ports
	networking := "bridge"
	if len(instance.Ports) > 0 {
		networking = "nat"
	}
	natRules := make([]nat.Rule, 0)
	for _, p := range instance.Ports {
		// TODO: Support for HostIP?
		// TODO: Do we need NAT or can we do Bridge? Use p.HostPort?
		hostPort, err := system.AvailablePort()
		if err == nil {
			natRules = append(natRules, nat.Rule{
				HostPort:  strconv.FormatInt(int64(hostPort), 10),
				GuestPort: strconv.FormatInt(int64(p.ContainerPort), 10),
			})
		} else {
			log.G(b.context).Error(errors.Wrap(err, "osv backend"))
		}
	}
	// Container.EnvFrom (TODO)
	// Container.Env (TODO)
	// Container.Resources (TODO)
	memory := 1024
	cpus := 1
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

	// Create hypervisor config
	switch imageConf.Hypervisor {
	case "qemu":
		dir := b.instanceDir(instanceID)
		conf := &qemu.VMConfig{
			Name:        instanceID,
			Verbose:     true,
			Cmd:         cmd,
			DisableKvm:  false,
			Persist:     false,
			InstanceDir: dir,
			Monitor:     b.instanceMoniPath(instanceID),
			ConfigFile:  b.instanceConfPath(instanceID),
			AioType:     b.repo.QemuAioType,
			Image:       image,
			BackingFile: true,
			Volumes:     []string{}, // TODO
			Memory:      int64(memory),
			Cpus:        cpus,
			Networking:  networking,
			Bridge:      "virbr0", // TODO
			NatRules:    natRules,
			MAC:         "", // TODO
			VNCFile:     b.instanceSockPath(instanceID),
		}
		if err = os.MkdirAll(dir, 0755); err != nil {
			return errors.Wrap(err, "osv")
		}
		if err = qemu.StoreConfig(conf); err != nil {
			return errors.Wrap(err, "osv")
		}
	default:
		err = errors.Errorf("platform %q is not supported", imageConf.Hypervisor)
		return errors.Wrap(err, "osv")
	}

	return nil
}

func (b *OSvBackend) StartInstance(instanceID string) error {
	instanceName, instancePlatform := capstan.SearchInstance(instanceID)
	if instanceName == "" {
		err := errors.Errorf("instance %q does not exist", instanceID)
		return errors.Wrap(err, "osv")
	}

	// Get config for platform
	var (
		cmd *exec.Cmd
		err error
	)
	switch instancePlatform {
	case "qemu":
		conf, err := qemu.LoadConfig(instanceName)
		if err != nil {
			return errors.Wrap(err, "osv")
		}
		cmd, err = qemu.VMCommand(conf, true)
		if err != nil {
			return errors.Wrap(err, "osv")
		}
	default:
		err = errors.Errorf("platform %q is not supported", instancePlatform)
		return errors.Wrap(err, "osv")
	}
	log.G(b.context).Infof("Started instance %q (backend=osv)", instanceID)
	logsFile, err := os.Create(b.instanceLogsPath(instanceID))
	if err != nil {
		return errors.Wrap(err, "osv")
	}
	r, w, _ := os.Pipe()
	cmd.Stdout, cmd.Stderr = w, w
	// Write everything to a log file continuously
	go func() { _, _ = io.Copy(logsFile, r) }()
	if err := cmd.Start(); err != nil {
		return errors.Wrap(err, "osv")
	}
	// Run goroutine that waits for the instance to exit
	go func() {
		err = cmd.Wait()
		// TODO: Put error in pod/container status?
		// Cleanup
		logsFile.Close()
	}()
	return nil
}

func (b *OSvBackend) UpdateInstance(instanceID string, instance corev1.Container) error {
	// TODO: Can we do this more performant?
	if err := b.DeleteInstance(instanceID); err != nil {
		return err
	}
	return b.CreateInstance(instanceID, instance)
}

func (b *OSvBackend) KillInstance(instanceID string, signal syscall.Signal) error {
	instanceName, instancePlatform := capstan.SearchInstance(instanceID)
	if instanceName == "" {
		err := errors.Errorf("instance %q does not exist", instanceID)
		return errors.Wrap(err, "osv")
	}

	// Stop instance with platform
	var err error
	switch instancePlatform {
	case "qemu":
		err = qemu.StopVM(instanceID)
	default:
		err = errors.Errorf("platform %q is not supported", instancePlatform)
		return errors.Wrap(err, "osv")
	}
	if err != nil {
		return errors.Wrap(err, "osv")
	}
	return nil
}

func (b *OSvBackend) DeleteInstance(instanceID string) error {
	instanceName, instancePlatform := capstan.SearchInstance(instanceID)
	if instanceName == "" {
		err := errors.Errorf("instance %q does not exist", instanceID)
		return errors.Wrap(err, "osv")
	}

	// Stop instance with platform
	var err error
	switch instancePlatform {
	case "qemu":
		qemu.StopVM(instanceID)
		err = qemu.DeleteVM(instanceID)
	default:
		err = errors.Errorf("platform %q is not supported", instancePlatform)
		return errors.Wrap(err, "osv")
	}
	if err != nil {
		return errors.Wrap(err, "osv")
	}
	return nil
}

func (b *OSvBackend) GetInstanceLogs(instanceID string, opts api.ContainerLogOpts) (io.ReadCloser, error) {
	// TODO: Check opts
	logsFile, err := os.Open(b.instanceLogsPath(instanceID))
	if err != nil {
		return nil, errors.Wrap(err, "osv")
	}
	return logsFile, nil
}

func (b *OSvBackend) RunInInstance(instanceID string, cmd []string, attach api.AttachIO) error {
	return nil
}

func (b *OSvBackend) CreateVolume(volumeID string, volume corev1.Volume) error {
	// Create volumes directory
	volumesDir := b.volumesDir()
	if _, err := os.Stat(volumesDir); os.IsNotExist(err) {
		if err = os.Mkdir(volumesDir, 0755); err != nil {
			return errors.Wrap(err, "osv")
		}
	}

	//// TODO: This is only required for local things
	//if volume.HostPath != nil && volume.HostPath.Type != nil {
	//	path := volume.HostPath.Path
	//}

	// TODO: hostDir with virtiofs?
	// https://github.com/cloudius-systems/osv/wiki/virtio-fs

	//// Create volume
	//volume.H
	//volumePath := b.volumePath(volumeID)
	//volumeSize := volume.()
	//if volumeSize == 0 {
	//	volumeSize = 512
	//}
	//if err := qemu.CreateVolume(volumePath, "raw", int64(volumeSize)); err != nil {
	//	return errors.Wrap(err, "osv")
	//}

	// TODO: Persist medata?
	return nil
}

func (b *OSvBackend) DeleteVolume(volumeID string) error {
	// Delete volume
	volumePath := b.volumePath(volumeID)
	if err := os.RemoveAll(volumePath); os.IsNotExist(err) {
		err = errors.Errorf("volume %q does not exist", volumeID)
		return errors.Wrap(err, "osv")
	}
	return nil
}

// Ensure interface is implemented
var _ Backend = (*OSvBackend)(nil)

func (b *OSvBackend) imageDir(imageRef string) string {
	cleaned := storage.CleanName(imageRef)
	cleaned = strings.ReplaceAll(cleaned, "/", "_")
	return filepath.Join(b.repo.RepoPath(), "fledge", cleaned)
}

func (b *OSvBackend) imageInfoPath(imageRef string) string {
	return filepath.Join(b.imageDir(imageRef), "index.yaml")
}

func (b *OSvBackend) imageConfPath(imageRef string) string {
	return filepath.Join(b.imageDir(imageRef), "config.yaml")
}

func (b *OSvBackend) imageDiskPath(imageRef string, hypervisor string) string {
	imageDir := b.imageDir(imageRef)
	imageBasePath := filepath.Join(imageDir, filepath.Base(imageDir))
	return fmt.Sprintf("%s.%s", imageBasePath, hypervisor)
}

func (b *OSvBackend) instanceDir(instanceID string) string {
	return filepath.Join(capstan.ConfigDir(), "instances/qemu", instanceID)
}

func (b *OSvBackend) instanceConfPath(instanceID string) string {
	return filepath.Join(b.instanceDir(instanceID), "osv.config")
}

func (b *OSvBackend) instanceMoniPath(instanceID string) string {
	return filepath.Join(b.instanceDir(instanceID), "osv.monitor")
}

func (b *OSvBackend) instanceSockPath(instanceID string) string {
	return filepath.Join(b.instanceDir(instanceID), "osv.socket")
}

func (b *OSvBackend) instanceLogsPath(instanceID string) string {
	return filepath.Join(b.instanceDir(instanceID), "osv.logs")
}

func (b *OSvBackend) volumesDir() string {
	return filepath.Join(capstan.ConfigDir(), "volumes/qemu")
}

func (b *OSvBackend) volumePath(volumeID string) string {
	volumesDir := b.volumesDir()
	return filepath.Join(volumesDir, volumeID)
}

// pullInstanceImage pulls the image into a local capstan repository
func (b *OSvBackend) pullInstanceImage(imageRef string, hypervisor string) error {
	imageDir := b.imageDir(imageRef)
	// Remove image if it exists
	if _, err := os.Stat(imageDir); !os.IsNotExist(err) {
		if err = os.RemoveAll(imageDir); err != nil {
			return err
		}
	}
	// Create repository directory
	if err := os.MkdirAll(imageDir, 0755); err != nil {
		return err
	}
	// Parse image reference
	r, err := ref.New(imageRef)
	if err != nil {
		return err
	}
	if r.Registry == "" {
		return fmt.Errorf("reference %s does not contain a valid registry", r.CommonName())
	}
	// Write image info to index.yaml
	imageInfo := capstan.ImageInfo{
		FormatVersion: "1",
		Version:       r.Tag,
		Created:       time.Now().Format(core.FRIENDLY_TIME_F),
		Description:   "OSv image imported by FLEDGE",
		Build:         "",
	}
	imageInfoFile, err := os.Create(b.imageInfoPath(imageRef))
	if err != nil {
		return err
	}
	imageInfoBytes, err := json.Marshal(imageInfo)
	if err != nil {
		return err
	}
	if _, err = imageInfoFile.Write(imageInfoBytes); err != nil {
		return err
	}
	//// Retrieve the image config
	rc := regclient.New(regclient.WithDockerCreds())
	//imageConf, err := storage.ImageGetConfigWithClient(rc, b.context, r)
	//if err != nil {
	//	return err
	//}
	//// Write image config to config.yaml
	//imageConfFile, err := os.Create(b.imageConfPath(imageRef))
	//if err != nil {
	//	return err
	//}
	//imageConfBytes, err := json.Marshal(imageConf)
	//if err != nil {
	//	return err
	//}
	//if _, err = imageConfFile.Write(imageConfBytes); err != nil {
	//	return err
	//}
	// Retrieve the image layers (should be one)
	layerDescs, err := storage.ImageGetLayersWithClient(rc, b.context, r)
	if err != nil {
		return err
	}
	if len(layerDescs) != 1 {
		return fmt.Errorf("image %s does not contain just one layer", r.CommonName())
	}
	// Check if the layer has the correct type
	layerDesc := layerDescs[0]
	if layerDesc.MediaType != types.MediaTypeOCI1Layer && layerDesc.MediaType != types.MediaTypeOCI1LayerGzip {
		return fmt.Errorf("layer media type %s is not supported", layerDesc.MediaType)
	}
	// Pull the stream
	layerBlob, err := rc.BlobGet(b.context, r, layerDesc)
	if err != nil {
		return err
	}
	layerTarReader, err := layerBlob.ToTarReader()
	if err != nil {
		return err
	}
	tr, err := layerTarReader.GetTarReader()
	if err != nil {
		return err
	}
	hdr, err := tr.Next()
	if err != nil {
		return err
	}
	if !strings.HasSuffix(hdr.Name, ".qemu") {
		return fmt.Errorf("unexpected file %s in layer of image %s", hdr.Name, r.CommonName())
	}
	// Write image layer to <base>.<hypervisor>
	imageDiskFile, err := os.Create(b.imageDiskPath(imageRef, hypervisor))
	if err != nil {
		return err
	}
	if _, err = io.Copy(imageDiskFile, tr); err != nil {
		return err
	}
	return nil
}
