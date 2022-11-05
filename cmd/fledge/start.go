package main

import (
	"context"
	"fmt"
	"github.com/virtual-kubelet/virtual-kubelet/log"
	"github.com/virtual-kubelet/virtual-kubelet/node"
	"github.com/virtual-kubelet/virtual-kubelet/node/nodeutil"
	"gitlab.ilabt.imec.be/fledge/service/pkg/config"
	"gitlab.ilabt.imec.be/fledge/service/pkg/provider"
	"net/http"
	"runtime"
	"time"
)

func Start(ctx context.Context, c *config.Config) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	newProvider := func(cfg nodeutil.ProviderConfig) (nodeutil.Provider, node.NodeProvider, error) {
		return provider.NewProvider(ctx, cfg)
	}

	mux := http.NewServeMux()
	cm, err := nodeutil.NewNode(c.DeviceName, newProvider, func(cfg *nodeutil.NodeConfig) error {
		// cfg.KubeconfigPath = c.KubeConfigPath
		cfg.Handler = mux
		// cfg.InformerResyncPeriod = c.InformerResyncPeriod

		// if taint != nil {
		// 	cfg.NodeSpec.Spec.Taints = append(cfg.NodeSpec.Spec.Taints, *taint)
		// }
		cfg.NodeSpec.Status.NodeInfo.Architecture = runtime.GOARCH
		cfg.NodeSpec.Status.NodeInfo.OperatingSystem = runtime.GOOS

		cfg.HTTPListenAddr = fmt.Sprintf(":%d", c.KubeletPort)
		// cfg.StreamCreationTimeout = apiConfig.StreamCreationTimeout
		// cfg.StreamIdleTimeout = apiConfig.StreamIdleTimeout
		cfg.DebugHTTP = true

		cfg.NumWorkers = 10 // c.PodSyncWorkers

		return nil
	},
		//setAuth(nodeName, apiConfig),
		nodeutil.WithTLSConfig(
			nodeutil.WithKeyPairFromPath("certificate.pem", "privatekey.pem"),               // TODO: Config var
			nodeutil.WithCAFromPath("/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"), // TODO: Config var
		),
		nodeutil.AttachProviderRoutes(mux),
	)
	if err != nil {
		return err
	}

	ctx = log.WithLogger(ctx, log.G(ctx).WithFields(log.Fields{
		// "provider":         c.Provider,
		"operatingSystem": runtime.GOOS,
		"node":            c.DeviceName,
		// "watchedNamespace": c.KubeNamespace,
	}))
	go cm.Run(ctx) //nolint:errcheck

	defer func() {
		log.G(ctx).Debug("Waiting for controllers to be done")
		cancel()
		<-cm.Done()
	}()

	log.G(ctx).Info("Waiting for controller to be ready")
	if err := cm.WaitReady(ctx, 60*time.Second); err != nil { // TODO: configure timeout
		return err
	}

	log.G(ctx).Info("Ready")

	select {
	case <-ctx.Done():
	case <-cm.Done():
		return cm.Err()
	}
	return nil
}

//func setAuth(node string, apiCfg *apiServerConfig) nodeutil.NodeOpt {
//	if apiCfg.CACertPath == "" {
//		return func(cfg *nodeutil.NodeConfig) error {
//			cfg.Handler = api.InstrumentHandler(nodeutil.WithAuth(nodeutil.NoAuth(), cfg.Handler))
//			return nil
//		}
//	}
//
//	return func(cfg *nodeutil.NodeConfig) error {
//		auth, err := nodeutil.WebhookAuth(cfg.Client, node, func(cfg *nodeutil.WebhookAuthConfig) error {
//			var err error
//			cfg.AuthnConfig.ClientCertificateCAContentProvider, err = dynamiccertificates.NewDynamicCAContentFromFile("ca-cert-bundle", apiCfg.CACertPath)
//			return err
//		})
//		if err != nil {
//			return err
//		}
//		cfg.Handler = api.InstrumentHandler(nodeutil.WithAuth(auth, cfg.Handler))
//		return nil
//	}
//}
