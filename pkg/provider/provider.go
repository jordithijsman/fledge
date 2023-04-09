package provider

import (
	"time"
)

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/virtual-kubelet/virtual-kubelet/log"
	"github.com/virtual-kubelet/virtual-kubelet/node/api"
	"github.com/virtual-kubelet/virtual-kubelet/node/api/statsv1alpha1"
	"github.com/virtual-kubelet/virtual-kubelet/node/nodeutil"
	"github.com/virtual-kubelet/virtual-kubelet/trace"
	"io"
	corev1 "k8s.io/api/core/v1"
	"os"
)

const (
	// Values used in tracing as attribute keys.
	namespaceKey     = "namespace"
	nameKey          = "name"
	containerNameKey = "containerName"
)

// Provider implements the virtual-kubelet provider interface and forwards calls to runtimes.
type Provider struct {
	nodeName           string
	operatingSystem    string
	internalIP         string
	daemonEndpointPort int32
	config             Config
	startTime          time.Time
	backends           map[string]Backend
	pods               map[string]*corev1.Pod
	instances          map[string]Instance
}

// NewProviderConfig creates a new Provider.
func NewProviderConfig(config Config, nodeName, operatingSystem string, internalIP string, daemonEndpointPort int32) (*Provider, error) {
	// set defaults
	if config.Default == "" {
		config.Default = defaultConfig.Default
	}
	if len(config.Enabled) == 0 {
		config.Enabled = defaultConfig.Enabled
	}
	// setup backend
	backends := map[string]Backend{}
	var err error
	for _, e := range config.Enabled {
		switch e {
		case BackendContainerd:
			if backends[e], err = NewDummyBackend(config); err != nil {
				return nil, err
			}
		case BackendOsv:
			if backends[e], err = NewDummyBackend(config); err != nil {
				return nil, err
			}
		default:
			return nil, errors.New(fmt.Sprintf("backend '%s' is not supported\n", e))
		}
	}

	// setup provider
	provider := Provider{
		nodeName:           nodeName,
		operatingSystem:    operatingSystem,
		internalIP:         internalIP,
		daemonEndpointPort: daemonEndpointPort,
		pods:               map[string]*corev1.Pod{},
		config:             config,
		startTime:          time.Now(),
		backends:           backends,
		instances:          map[string]Instance{},
	}
	return &provider, nil
}

// NewProvider creates a new Provider, which implements the PodNotifier interface
func NewProvider(providerConfig, nodeName, operatingSystem string, internalIP string, daemonEndpointPort int32) (*Provider, error) {
	cfg, err := loadConfig(providerConfig)
	if err != nil {
		return nil, err
	}

	return NewProviderConfig(cfg, nodeName, operatingSystem, internalIP, daemonEndpointPort)
}

// loadConfig loads the given json configuration files.
func loadConfig(providerConfig string) (cfg Config, err error) {
	data, err := os.ReadFile(providerConfig)
	if err != nil {
		return cfg, err
	}
	err = json.Unmarshal(data, &cfg)
	if err != nil {
		return cfg, err
	}
	if cfg.Default == "" {
		cfg.Default = defaultConfig.Default
	}
	if len(cfg.Enabled) == 0 {
		cfg.Enabled = defaultConfig.Enabled
	}
	return cfg, nil
}

// GetContainerLogs retrieves the logs of a container by name from the provider.
func (p *Provider) GetContainerLogs(ctx context.Context, namespace, podName, containerName string, opts api.ContainerLogOpts) (io.ReadCloser, error) {
	ctx, span := trace.StartSpan(ctx, "GetContainerLogs")
	defer span.End()

	// Add pod and container attributes to the current span.
	ctx = addAttributes(ctx, span, namespaceKey, namespace, nameKey, podName, containerNameKey, containerName)

	log.G(ctx).Info("receive GetContainerLogs %q", podName)

	// TODO: return p.runtime.GetContainerLogs(namespace, podName, containerName, opts)
	return nil, errors.New("GetContainerLogs not implemented")
}

// GetStatsSummary gets the stats for the node, including running pods
func (p *Provider) GetStatsSummary(ctx context.Context) (*statsv1alpha1.Summary, error) {
	ctx, span := trace.StartSpan(ctx, "GetStatsSummary")
	defer span.End()

	log.G(ctx).Info("receive GetStatsSummary")

	// TODO Implement

	return nil, errors.New("GetStatsSummary not implemented")
}

// TODO: Implement NodeChanged for performance reasons

// Ensure interface is implemented
var _ nodeutil.Provider = (*Provider)(nil)
