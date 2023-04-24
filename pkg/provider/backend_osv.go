package provider

import (
	"bytes"
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
	"gitlab.ilabt.imec.be/fledge/service/pkg/manager"
	"gitlab.ilabt.imec.be/fledge/service/pkg/storage"
	"gitlab.ilabt.imec.be/fledge/service/pkg/system"
	"gitlab.ilabt.imec.be/fledge/service/pkg/util"
	"golang.org/x/net/context"
	"io"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"text/template"
	"time"

	capstan "github.com/cloudius-systems/capstan/util"
	"github.com/virtual-kubelet/virtual-kubelet/node/api"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
)

type OSvExtras struct {
	// vmProc specify extra processes (e.g. daemons) that needs to be started before the vm
	vmProc [][]string
	// vmArgs specify extra arguments (e.g. mounts) that need to be passed to the hypervisor
	vmArgs []string
	// vmOpts specify extra options (e.g. mounts) that need to be passed to the virtual machine
	// the options can be templated by using Go template syntax for a VolumeMount
	vmOpts []string
}

func (e *OSvExtras) extendWith(other *OSvExtras) {
	e.vmProc = append(e.vmProc, other.vmProc...)
	e.vmArgs = append(e.vmArgs, other.vmArgs...)
	e.vmOpts = append(e.vmOpts, other.vmOpts...)
}

func (e *OSvExtras) executeTemplate(index int, mount *corev1.VolumeMount) error {
	for i, vmOpt := range e.vmOpts {
		tmpl, err := template.New("VolumeMount").Parse(vmOpt)
		if err != nil {
			return err
		}
		var b bytes.Buffer
		d := struct {
			Index int
			Mount *corev1.VolumeMount
		}{Index: index, Mount: mount}
		if err = tmpl.Execute(&b, d); err != nil {
			return err
		}
		e.vmOpts[i] = b.String()
	}
	return nil
}

type OSvBackend struct {
	config          Config
	resourceManager *manager.ResourceManager

	context context.Context
	repo    *capstan.Repo

	instanceStatuses map[string]*corev1.ContainerStatus
	instanceExtras   map[string]*OSvExtras
	volumeExtras     map[string]*OSvExtras
}

func NewOSvBackend(cfg Config, resourceManager *manager.ResourceManager) (*OSvBackend, error) {
	repo := capstan.NewRepo("")

	b := &OSvBackend{
		config:           cfg,
		resourceManager:  resourceManager,
		context:          context.Background(),
		repo:             repo,
		instanceStatuses: map[string]*corev1.ContainerStatus{},
		instanceExtras:   map[string]*OSvExtras{},
		volumeExtras:     map[string]*OSvExtras{},
	}

	return b, nil
}

func (b *OSvBackend) GetInstanceStatus(instanceID string) (corev1.ContainerStatus, error) {
	if instanceStatus, ok := b.instanceStatuses[instanceID]; ok {
		return *instanceStatus, nil
	}
	err := errors.Errorf("instance %q does not exist", instanceID)
	return corev1.ContainerStatus{}, errors.Wrap(err, "osv")
}

func (b *OSvBackend) CreateInstance(instanceID string, instance corev1.Container) error {
	// Clean up pre-existing instance (TODO can we do this in SIGKILL or on startup?)
	// Otherwise capstan will start a pre-existing instance that was stopped
	b.DeleteInstance(instanceID)

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
	cmd := append(instance.Command, instance.Args...)
	if cmd == nil || len(cmd) == 0 {
		cmd = []string{"runscript", "/run/default;"}
	}
	cmd = append([]string{"--verbose"}, cmd...) // TODO: If verbose setting?
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
	// Container.VolumeMounts
	instanceExtras := &OSvExtras{}
	for i, vm := range instance.VolumeMounts {
		parts := splitIdentifierIntoParts(instanceID)
		parts[len(parts)-1] = vm.Name
		volumeID := joinIdentifierFromParts(parts...)
		volumeExtras, ok := b.volumeExtras[volumeID]
		if !ok {
			err = errors.Errorf("volume %q was not created", volumeID)
			return errors.Wrap(err, "osv")
		}
		if err = volumeExtras.executeTemplate(i, &vm); err != nil {
			return errors.Wrap(err, "osv")
		}
		instanceExtras.extendWith(volumeExtras)
	}
	cmd = append(instanceExtras.vmOpts, cmd...)
	b.instanceExtras[instanceID] = instanceExtras
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

	// Show command in logs
	log.G(b.context).Infof("Setting cmdline: %s\n", strings.Join(cmd, " "))

	// Create hypervisor config
	switch imageConf.Hypervisor {
	case "qemu":
		dir := b.instanceDir(instanceID)
		conf := &qemu.VMConfig{
			Name:        instanceID,
			Verbose:     true,
			Cmd:         strings.Join(cmd, " "),
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

	// ContainerStatus.Name
	parts := splitIdentifierIntoParts(instanceID)
	name := parts[len(parts)-1]
	// ContainerStatus.State
	state := corev1.ContainerState{
		Waiting: &corev1.ContainerStateWaiting{
			Reason:  "Created",
			Message: "Instance is created",
		},
	}
	// ContainerStatus.LastTerminationState
	lastTerminationState := corev1.ContainerState{}
	if instanceStatus, ok := b.instanceStatuses[instanceID]; ok {
		lastTerminationState = instanceStatus.State
	}
	// ContainerStatus.Started
	started := false
	// Instance is created, populate its status
	// https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.23/#containerstatus-v1-core
	b.instanceStatuses[instanceID] = &corev1.ContainerStatus{
		Name:                 name,
		State:                state,
		LastTerminationState: lastTerminationState,
		Ready:                true, // Default value
		RestartCount:         0,    // Default value
		Image:                instance.Image,
		ImageID:              "",
		ContainerID:          fmt.Sprintf("capstan://%s", instanceID),
		Started:              &started,
	}

	return nil
}

func (b *OSvBackend) StartInstance(instanceID string) error {
	instanceName, instancePlatform := capstan.SearchInstance(instanceID)
	if instanceName == "" {
		err := errors.Errorf("instance %q does not exist", instanceID)
		return errors.Wrap(err, "osv")
	}

	// Get extras for instance
	extras, ok := b.instanceExtras[instanceID]
	if !ok {
		err := errors.Errorf("instance %q does not have extras", instanceID)
		return errors.Wrap(err, "osv")
	}

	// Get config for platform
	var (
		cmd  *exec.Cmd
		pErr error
	)
	switch instancePlatform {
	case "qemu":
		conf, err := qemu.LoadConfig(instanceName)
		if err != nil {
			return errors.Wrap(err, "osv")
		}
		cmd, err = qemu.VMCommand(conf, true, extras.vmArgs...)
		if err != nil {
			return errors.Wrap(err, "osv")
		}
	default:
		pErr = errors.Errorf("platform %q is not supported", instancePlatform)
		return errors.Wrap(pErr, "osv")
	}
	r, w, _ := os.Pipe()
	// Start side processes
	ctx, cancel := context.WithCancel(b.context)
	procs := make([]*exec.Cmd, 0)
	for _, p := range extras.vmProc {
		proc := exec.CommandContext(ctx, p[0], p[1:]...)
		proc.Stdout, proc.Stderr = w, w
		if err := proc.Start(); err != nil {
			cancel()
			return errors.Wrap(err, "osv")
		}
		go func() {
			pErr = proc.Wait()
			_, exitMsg := util.ExecParseError(proc.Wait())
			log.G(ctx).Debugf("process %q %s\n", proc.String(), exitMsg)
		}()
		procs = append(procs, proc)
	}
	// Create logfile
	log.G(b.context).Infof("Started instance %q (backend=osv)", instanceID)
	logsFile, pErr := os.Create(b.instanceLogsPath(instanceID))
	if pErr != nil {
		return errors.Wrap(pErr, "osv")
	}
	cmd.Stdout, cmd.Stderr = w, w
	// Write everything to a log file continuously
	go func() { _, _ = io.Copy(logsFile, r) }()
	if err := cmd.Start(); err != nil {
		return errors.Wrap(err, "osv")
	}

	// Instance is started, update its status
	// https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.23/#containerstaterunning-v1-core
	instanceStatus := b.instanceStatuses[instanceID]
	instanceStatus.State = corev1.ContainerState{
		Running: &corev1.ContainerStateRunning{
			StartedAt: metav1.NewTime(time.Now()),
		},
	}

	// Run goroutine that waits for the instance to exit
	go func() {
		exitCode, exitMsg := util.ExecParseError(cmd.Wait())
		log.G(ctx).Debugf("instance %s\n", exitMsg)
		// Cancel subprocesses
		cancel()
		// Close logfile stream
		logsFile.Close()
		// Instance has terminated, update its status
		// https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.23/#containerstateterminated-v1-core
		instanceStatus.State = corev1.ContainerState{
			Terminated: &corev1.ContainerStateTerminated{
				ExitCode:    int32(exitCode),
				Signal:      int32(syscall.SIGTERM), // Default
				Reason:      "Terminated",
				Message:     exitMsg,
				StartedAt:   instanceStatus.State.Running.StartedAt,
				FinishedAt:  metav1.NewTime(time.Now()),
				ContainerID: instanceID,
			},
		}
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

	// Instance is deleted, remove its status (TODO: last termination state)
	delete(b.instanceStatuses, instanceID)

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
		if err = os.MkdirAll(volumesDir, 0755); err != nil {
			return errors.Wrap(err, "osv")
		}
	}

	parts := splitIdentifierIntoParts(volumeID)
	namespace := parts[0]

	switch {
	case volume.ConfigMap != nil:
		// TODO: This is ugly, can we make this more generic? Maybe a custom kind of volume?
		// If this is a custom kind of volume, are we maybe able to redefine volume mounts in corev1.Container
		// by making an Instance struct that inherits from it?
		configMap, err := b.resourceManager.GetConfigMap(volume.ConfigMap.Name, namespace)
		if volume.ConfigMap.Optional != nil && !*volume.ConfigMap.Optional && k8serrors.IsNotFound(err) {
			return fmt.Errorf("configMap %s is required by volume %q and does not exist", volume.ConfigMap.Name, volumeID)
		}
		if configMap == nil {
			return nil
		}

	case volume.HostPath != nil:
		switch *volume.HostPath.Type {
		case corev1.HostPathDirectoryOrCreate:
			if err := os.MkdirAll(volume.HostPath.Path, 0755); err != nil {
				return errors.Wrap(err, "osv")
			}
			fallthrough
		case corev1.HostPathDirectory:
		default:
			err := errors.Errorf("volume %q has unsupported hostPath.type %q", volumeID, volume.HostPath.Type)
			return errors.Wrap(err, "osv")
		}
		/*
			Create options for virtio-fs socket according to scripts/run.py
			https://raw.githubusercontent.com/cloudius-systems/osv/master/scripts/run.py
			https://github.com/cloudius-systems/osv/wiki/virtio-fs
		*/
		// Create temporary file for virtio-fs socket
		socketPath := filepath.Join(volumesDir, fmt.Sprintf("%s.sock", volumeID))
		// Determine arguments for virtio-fs
		memSize := "1G" // TODO: configure
		vmProc := []string{
			"virtiofsd",
			"--socket-path", socketPath,
			"--shared-dir", volume.HostPath.Path,
			"--no-announce-submounts",
		}
		vmArgs := []string{
			"-chardev", fmt.Sprintf("socket,id=char0,path=%s", socketPath),
			"-device", fmt.Sprintf("vhost-user-fs-pci,queue-size=1024,chardev=char0,tag=%s", volume.Name),
			"-object", fmt.Sprintf("memory-backend-file,id=mem,size=%s,mem-path=/dev/shm,share=on", memSize),
			"-numa", "node,memdev=mem",
		}
		vmOpts := []string{
			"--rootfs=zfs",
			"--mount-fs=virtiofs,/dev/virtiofs{{ .Index }},{{ .Mount.MountPath }}",
		}
		b.volumeExtras[volumeID] = &OSvExtras{
			vmProc: [][]string{vmProc},
			vmArgs: vmArgs,
			vmOpts: vmOpts,
		}
	case volume.Projected != nil:
		// TODO: Solve this with another virtio-fs mount?
		for _, source := range volume.Projected.Sources {
			switch {
			case source.ServiceAccountToken != nil:
				// TODO
			case source.Secret != nil:
				// TODO
			case source.ConfigMap != nil:
				// TODO
			}
		}
		// TODO: We stub projected volumes for now
		b.volumeExtras[volumeID] = &OSvExtras{}
	default:
		err := errors.Errorf("unsupported volume %q", volumeID)
		return errors.Wrap(err, "osv")
	}
	return nil
}

func (b *OSvBackend) UpdateVolume(volumeID string, volume corev1.Volume) error {
	// TODO: Can we do this more performant?
	if err := b.DeleteVolume(volumeID); err != nil {
		return err
	}
	return b.CreateVolume(volumeID, volume)
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
