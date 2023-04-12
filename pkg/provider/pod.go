package provider

import (
	"context"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/reference/docker"
	"github.com/pkg/errors"
	"github.com/virtual-kubelet/virtual-kubelet/trace"
	"gitlab.ilabt.imec.be/fledge/service/pkg/storage"
	corev1 "k8s.io/api/core/v1"
)

// GetPod retrieves a pod by name from the provider (can be cached).
// The Pod returned is expected to be immutable, and may be accessed
// concurrently outside of the calling goroutine. Therefore it is recommended
// to return a version after DeepCopy.
func (p *Provider) GetPod(ctx context.Context, namespace, name string) (*corev1.Pod, error) {
	ctx, span := trace.StartSpan(ctx, "GetPod")
	defer span.End()

	// Add the pod's coordinates to the current span.
	ctx = addAttributes(ctx, span, namespaceKey, namespace, nameKey, name)

	log.G(ctx).Debugf("receive GetPod %q", name)

	pod, found := p.pods[joinIdentifierFromParts(namespace, name)]
	if !found {
		return nil, errors.Errorf("Pod %s/%s not found", namespace, name)
	}
	return pod, nil
}

// GetPods retrieves a list of all pods running on the provider (can be cached).
// The Pods returned are expected to be immutable, and may be accessed
// concurrently outside of the calling goroutine. Therefore it is recommended
// to return a version after DeepCopy.
func (p *Provider) GetPods(ctx context.Context) ([]*corev1.Pod, error) {
	ctx, span := trace.StartSpan(ctx, "GetPods")
	defer span.End()

	log.G(ctx).Info("receive GetPods")

	var pods []*corev1.Pod
	for _, pod := range p.pods {
		pods = append(pods, pod)
	}
	return pods, nil
}

// CreatePod takes a Kubernetes Pod and deploys it within the provider.
func (p *Provider) CreatePod(ctx context.Context, pod *corev1.Pod) error {
	ctx, span := trace.StartSpan(ctx, "CreatePod")
	defer span.End()

	// Add the pod's coordinates to the current span.
	ctx = addAttributes(ctx, span, namespaceKey, pod.Namespace, nameKey, pod.Name)

	log.G(ctx).Debugf("receive CreatePod %q", pod.Name)

	// Register pod specification
	podID := podToIdentifier(pod)
	p.pods[podID] = pod

	// Create volumes
	//vol, err := p.volumes(pod, volumeAll)
	//if err != nil {
	//	err = errors.Join(err, errors.New("failed to process Pod volumes"))
	//	log.G(ctx).Error(err)
	//	return err
	//}

	// Get Uid and Gid
	//uid, gid, err := uidGidFromSecurityContext(pod, p.config.OverrideRootUID)
	//if err != nil {
	//	return err
	//}

	//tmpfs := strings.Join([]string{"/var", "/run"}, " ")
	//
	//previousUnit := ""
	var instancesToStart []Instance
	for i, c := range append(pod.Spec.InitContainers, pod.Spec.Containers...) {
		isInit := i < len(pod.Spec.InitContainers)
		log.G(ctx).Debugf("processing container %d (init=%t)", i, isInit)

		// Check for name collision just in case
		instanceID := podAndContainerToIdentifier(pod, c)
		if _, ok := p.instances[instanceID]; ok {
			return errors.Errorf("name collision for instance %q", instanceID)
		}

		// Convert image name to something universal
		imageRef, err := docker.ParseDockerRef(c.Image)
		if err != nil {
			return errors.Errorf("failed parsing reference %q", c.Image)
		}
		c.Image = imageRef.String()

		// Get the config of the image to determine the backend
		im, err := storage.ImageGetConfig(ctx, c.Image)
		if err != nil {
			return errors.Wrapf(err, "unable to get image config of %q", c.Image)
		}
		// If no backend is defined, it's a non-FLEDGE and should be run with containerd
		if im.Backend == "" {
			im.Backend = BackendContainerd
		}
		// Check if the required backend is enabled
		be := p.backends[im.Backend]
		if be == nil {
			return errors.Wrapf(err, "failed to find enabled backend %q", im.Backend)
		}
		// Create instance
		log.G(ctx).Infof("creating instance %q (backend=%s)", instanceID, im.Backend)
		err = be.CreateInstance(instanceID, c)
		if err != nil {
			return errors.Wrapf(err, "failed to create instance %q", instanceID)
		}
		instance := Instance{ID: instanceID, Backend: be}
		instancesToStart = append(instancesToStart, instance)
		p.instances[instanceID] = instance

		//bindmounts := []string{}
		//bindmountsro := []string{}
		//rwpaths := []string{}
		//for _, v := range c.VolumeMounts {
		//	dir, ok := vol[v.Name]
		//	if !ok {
		//		fnlog.Warnf("failed to find volumeMount %s in the specific volumes, skipping", v.Name)
		//		continue
		//	}
		//
		//	if v.ReadOnly {
		//		bindmountsro = append(bindmountsro, fmt.Sprintf("%s:%s", dir, v.MountPath)) // SubPath, look at todo, filepath.Join?
		//		continue
		//	}
		//	rwpaths = append(rwpaths, v.MountPath)
		//	if dir == "" { // hostPath
		//		continue
		//	}
		//	bindmounts = append(bindmounts, fmt.Sprintf("%s:%s", dir, v.MountPath)) // SubPath, look at todo, filepath.Join?
		//	// OK, so the v.MountPath will _exist_ on the system, as systemd will create it, permissions should not matter, as we
		//	// only need this "hook" to mount the bindmount.
		//}
		//
		//c.Image = ospkg.Clean(c.Image) // clean up the image if fetched with http(s)
		//name := podToUnitName(pod, c.Name)
		//if installed {
		//	p.unitManager.Mask(c.Image + unit.ServiceSuffix)
		//}
		//
		//uf, err := p.unitfileFromPackageOrSynthesized(c)
		//if err != nil {
		//	err = errors.Wrapf(err, "failed to process unit file for %q", c.Image)
		//	fnlog.Error(err)
		//	return err
		//}
		//if c.WorkingDir != "" {
		//	uf = uf.Overwrite("Service", "WorkingDirectory", c.WorkingDir)
		//}
		//
		//uf = uf.Overwrite("Service", "ProtectSystem", "true")
		//uf = uf.Overwrite("Service", "ProtectHome", "tmpfs")
		//uf = uf.Overwrite("Service", "PrivateMounts", "true")
		//uf = uf.Overwrite("Service", "ReadOnlyPaths", "/")
		//uf = uf.Insert("Service", "StandardOutput", "journal")
		//uf = uf.Insert("Service", "StandardError", "journal")
		//
		//// User/group handling. If the podspec has a security context we use that. This takes into acount the --override-root-uid flag value.
		//// If these are not set, the unit file's value are used. Note if the unit file doesn't specify it, it *defaults*
		//// to root, but we only care about that when a root override is set.
		//hasRoot := false
		//unitUser := uf.Contents["Service"]["User"]
		//if len(unitUser) == 0 || unitUser[0] == "0" || unitUser[0] == "root" {
		//	hasRoot = true
		//}
		//unitGroup := uf.Contents["Service"]["Group"] // User=1, and a Group=0|root is also considered unwanted
		//if len(unitGroup) == 0 || unitGroup[0] == "0" || unitGroup[0] == "root" {
		//	hasRoot = true
		//}
		//if uid == "" && hasRoot && p.config.OverrideRootUID > 0 {
		//	mapuid := strconv.FormatInt(int64(p.config.OverrideRootUID), 10)
		//	u, err := user.LookupId(mapuid)
		//	if err != nil {
		//		return fmt.Errorf("root override UID %q, not found: %s", mapuid, err)
		//	}
		//	uid = u.Uid
		//	gid = u.Gid
		//}
		//
		//if uid != "" {
		//	uf = uf.Overwrite("Service", "User", uid)
		//	uf = uf.Overwrite("Service", "Group", gid)
		//}
		//
		//// Treat initContainer differently.
		//if isInit {
		//	uf = uf.Overwrite("Service", "Type", "oneshot") // no restarting
		//	uf = uf.Insert(kubernetesSection, "InitContainer", "true")
		//}
		//
		//// Handle unit dependencies.
		//if previousUnit != "" {
		//	uf = uf.Insert("Unit", "After", previousUnit)
		//}
		//
		//// keep the unit around, until DeletePod is triggered.
		//// this is also for us to return the state even after the unit left the stage.
		//uf = uf.Overwrite("Service", "RemainAfterExit", "true")
		//
		//execStart := commandAndArgs(uf, c)
		//if len(execStart) > 0 {
		//	uf = uf.Overwrite("Service", "ExecStart", strings.Join(execStart, " "))
		//}
		//
		//id := string(pod.ObjectMeta.UID) // give multiple containers the same access? Need to test this.
		//uf = uf.Insert(kubernetesSection, "Namespace", pod.ObjectMeta.Namespace)
		//uf = uf.Insert(kubernetesSection, "ClusterName", pod.ObjectMeta.ClusterName)
		//uf = uf.Insert(kubernetesSection, "Id", id)
		//uf = uf.Insert(kubernetesSection, "Image", c.Image) // save (cleaned) image name here, we're not tracking this in the unit's name.
		//
		//uf = uf.Insert("Service", "TemporaryFileSystem", tmpfs)
		//if len(rwpaths) > 0 {
		//	paths := strings.Join(rwpaths, " ")
		//	uf = uf.Insert("Service", "ReadWritePaths", paths)
		//}
		//if len(bindmounts) > 0 {
		//	mount := strings.Join(bindmounts, " ")
		//	uf = uf.Insert("Service", "BindPaths", mount)
		//}
		//if len(bindmountsro) > 0 {
		//	romount := strings.Join(bindmountsro, " ")
		//	uf = uf.Insert("Service", "BindReadOnlyPaths", romount)
		//}
		//
		//for _, del := range deleteOptions {
		//	uf = uf.Delete("Service", del)
		//}
		//
		//envVars := p.defaultEnvironment()
		//for _, env := range c.Env {
		//	// If environment variable is a string with spaces, it must be quoted.
		//	// Quoting seems innocuous to other strings so it's set by default.
		//	envVars = append(envVars, fmt.Sprintf("%s=%q", env.Name, env.Value))
		//}
		//for _, env := range envVars {
		//	uf = uf.Insert("Service", "Environment", env)
		//}
		//
		//// For logging purposes only.
		//init := ""
		//if isInit {
		//	init = "init-"
		//}
		//fnlog.Infof("loading %sunit %q, %q as %q\n%s", init, c.Name, c.Image, name, uf)
		//if err := p.unitManager.Load(name, *uf); err != nil {
		//	fnlog.Errorf("failed to load unit %q: %s", name, err)
		//}
		//unitsToStart = append(unitsToStart, name)
		//if isInit {
		//	previousUnit = name
		//}
	}
	for _, instance := range instancesToStart {
		log.G(ctx).Infof("starting instance %q", instance.ID)
		if err := instance.Start(); err != nil {
			log.G(ctx).Errorf("failed to start instance %q: %s", instance.ID, err)
		}
	}
	//p.podResourceManager.Watch(pod)
	return nil
}

// GetPodStatus retrieves the status of a pod by name from the provider.
// The PodStatus returned is expected to be immutable, and may be accessed
// concurrently outside of the calling goroutine. Therefore it is recommended
// to return a version after DeepCopy.
func (p *Provider) GetPodStatus(ctx context.Context, namespace, name string) (*corev1.PodStatus, error) {
	ctx, span := trace.StartSpan(ctx, "GetPodStatus")
	defer span.End()

	// Add namespace and name as attributes to the current span.
	ctx = addAttributes(ctx, span, namespaceKey, namespace, nameKey, name)

	log.G(ctx).Debugf("receive GetPodStatus %q", name)

	// Return status field of pod (TODO: Update the actual status constantly)
	pod, err := p.GetPod(ctx, namespace, name)
	if err != nil {
		return nil, err
	}
	return &pod.Status, nil
}

// UpdatePod takes a Kubernetes Pod and updates it within the provider.
func (p *Provider) UpdatePod(ctx context.Context, pod *corev1.Pod) error {
	ctx, span := trace.StartSpan(ctx, "UpdatePod")
	defer span.End()

	// Add the pod's coordinates to the current span.
	ctx = addAttributes(ctx, span, namespaceKey, pod.Namespace, nameKey, pod.Name)

	log.G(ctx).Debugf("receive UpdatePod %q", pod.Name)

	// TODO: backend.UpdateContainer
	return errors.Errorf("UpdatePod not implemented")
}

// DeletePod takes a Kubernetes Pod and deletes it from the provider. Once a pod is deleted, the provider is
// expected to call the NotifyPods callback with a terminal pod status where all the containers are in a terminal
// state, as well as the pod. DeletePod may be called multiple times for the same pod.
func (p *Provider) DeletePod(ctx context.Context, pod *corev1.Pod) error {
	ctx, span := trace.StartSpan(ctx, "DeletePod")
	defer span.End()

	// Add the pod's coordinates to the current span.
	ctx = addAttributes(ctx, span, namespaceKey, pod.Namespace, nameKey, pod.Name)

	log.G(ctx).Debugf("receive DeletePod %q", pod.Name)

	// TODO: return p.runtime.DeletePod(pod)
	return errors.Errorf("DeletePod not implemented")
}

func (p *Provider) getInstance(namespace string, podName string, containerName string) (Instance, bool) {
	instanceID := joinIdentifierFromParts(namespace, podName, containerName)
	instance, ok := p.instances[instanceID]
	return instance, ok
}
