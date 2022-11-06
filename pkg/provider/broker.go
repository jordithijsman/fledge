package provider

import (
	"context"
	"errors"
	"github.com/virtual-kubelet/virtual-kubelet/node"
	"github.com/virtual-kubelet/virtual-kubelet/node/api"
	"github.com/virtual-kubelet/virtual-kubelet/node/api/statsv1alpha1"
	"github.com/virtual-kubelet/virtual-kubelet/node/nodeutil"
	"github.com/virtual-kubelet/virtual-kubelet/trace"
	"gitlab.ilabt.imec.be/fledge/service/pkg/util"
	"io"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"runtime"
)

type BrokerProvider struct {
}

func NewProvider(ctx context.Context, cfg nodeutil.ProviderConfig) (nodeutil.Provider, node.NodeProvider, error) {
	p := &BrokerProvider{}
	p.ConfigureNode(ctx, cfg.Node)
	// Read KubeletVersion from BuildInfo
	version, _ := util.ReadDepVersion("k8s.io/api")
	cfg.Node.Status.NodeInfo.KubeletVersion = version
	return p, nil, nil
}

func (p *BrokerProvider) ConfigureNode(ctx context.Context, n *corev1.Node) { //nolint:golint
	ctx, span := trace.StartSpan(ctx, "mock.ConfigureNode") //nolint:staticcheck,ineffassign
	defer span.End()
	capacity := corev1.ResourceList{
		corev1.ResourceCPU:    resource.MustParse("1000m"),
		corev1.ResourceMemory: resource.MustParse("4Gi"),
		corev1.ResourcePods:   resource.MustParse("10"),
	}
	n.Status.Capacity = capacity
	n.Status.Allocatable = capacity
	//n.Status.Conditions = p.nodeConditions()
	//n.Status.Addresses = p.nodeAddresses()
	//n.Status.DaemonEndpoints = p.nodeDaemonEndpoints()
	n.Status.NodeInfo.OperatingSystem = runtime.GOOS
	n.Status.NodeInfo.Architecture = runtime.GOARCH
	n.ObjectMeta.Labels["alpha.service-controller.kubernetes.io/exclude-balancer"] = "true"
	n.ObjectMeta.Labels["node.kubernetes.io/exclude-from-external-load-balancers"] = "true"
}

/* begin nodeutil.Provider */

// CreatePod takes a Kubernetes Pod and deploys it within the provider.
func (p *BrokerProvider) CreatePod(ctx context.Context, pod *corev1.Pod) error {
	return errors.New("CreatePod not implemented")
}

// UpdatePod takes a Kubernetes Pod and updates it within the provider.
func (p *BrokerProvider) UpdatePod(ctx context.Context, pod *corev1.Pod) error {
	return errors.New("UpdatePod not implemented")
}

// DeletePod takes a Kubernetes Pod and deletes it from the provider. Once a pod is deleted, the provider is
// expected to call the NotifyPods callback with a terminal pod status where all the containers are in a terminal
// state, as well as the pod. DeletePod may be called multiple times for the same pod.
func (p *BrokerProvider) DeletePod(ctx context.Context, pod *corev1.Pod) error {
	return errors.New("DeletePod not implemented")
}

// GetPod retrieves a pod by name from the provider (can be cached).
// The Pod returned is expected to be immutable, and may be accessed
// concurrently outside of the calling goroutine. Therefore it is recommended
// to return a version after DeepCopy.
func (p *BrokerProvider) GetPod(ctx context.Context, namespace, name string) (*corev1.Pod, error) {
	return nil, errors.New("GetPod not implemented")
}

// GetPodStatus retrieves the status of a pod by name from the provider.
// The PodStatus returned is expected to be immutable, and may be accessed
// concurrently outside of the calling goroutine. Therefore it is recommended
// to return a version after DeepCopy.
func (p *BrokerProvider) GetPodStatus(ctx context.Context, namespace, name string) (*corev1.PodStatus, error) {
	return nil, errors.New("GetPodStatus not implemented")
}

// GetPods retrieves a list of all pods running on the provider (can be cached).
// The Pods returned are expected to be immutable, and may be accessed
// concurrently outside of the calling goroutine. Therefore it is recommended
// to return a version after DeepCopy.
func (p *BrokerProvider) GetPods(context.Context) ([]*corev1.Pod, error) {
	// return make([]*corev1.Pod, 0), nil
	return nil, errors.New("GetPods not implemented")
}

// GetContainerLogs retrieves the logs of a container by name from the provider.
func (p *BrokerProvider) GetContainerLogs(ctx context.Context, namespace, podName, containerName string, opts api.ContainerLogOpts) (io.ReadCloser, error) {
	return nil, errors.New("GetContainerLogs not implemented")
}

// RunInContainer executes a command in a container in the pod, copying data
// between in/out/err and the container's stdin/stdout/stderr.
func (p *BrokerProvider) RunInContainer(ctx context.Context, namespace, podName, containerName string, cmd []string, attach api.AttachIO) error {
	return errors.New("RunInContainer not implemented")
}

// GetStatsSummary gets the stats for the node, including running pods
func (p *BrokerProvider) GetStatsSummary(context.Context) (*statsv1alpha1.Summary, error) {
	return nil, errors.New("GetStatsSummary not implemented")
}

// Ensure interface is implemented
var _ nodeutil.Provider = (*BrokerProvider)(nil)

/* end nodeutil.Provider */

//// NodeAddresses returns a list of addresses for the node status
//// within Kubernetes.
//func (p *BrokerProvider) nodeAddresses(ctx context.Context) []v1.NodeAddress {
//	nodenameStr, _ := manager.ExecCmdBash("hostname")
//	p.nodeName = strings.TrimSuffix(nodenameStr, "\n")
//	addresshost := v1.NodeAddress{Type: v1.NodeHostName, Address: p.nodeName}
//	addressip := v1.NodeAddress{Type: v1.NodeInternalIP, Address: config.Cfg.DeviceIP}
//	addresses := []v1.NodeAddress{addresshost, addressip}
//	return addresses
//}
